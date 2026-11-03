#!/bin/sh
# The host side of `make smoke` and `make scenario`: boot each run that the
# runs file ($1, which `tools smoke prepare` or `tools scenario prepare`
# writes) lists in PCSX-Redux ($PCSX_REDUX) with tools/emulator.lua and the
# run's script, and close an emulator whose log.txt stops for
# $EMULATOR_WATCHDOG seconds. A line gives the script (smoke, scenario), the
# run's directory, the cue sheet, the memory card slot 1 takes from an
# earlier run (- for none; the run is skipped when that card holds no save of
# the game) and the argument of the script's entry, SCRIPT_run(DIR, ARGUMENT).
# Under WSL the emulator is a Windows program: it gets Windows paths, and the
# watchdog closes the pcsx-redux.main process that its launcher started.
set -u
: "${PCSX_REDUX:?set PCSX_REDUX to the PCSX-Redux executable}"
runs=${1:?usage: tools/emulator.sh RUNS}
watchdog=${EMULATOR_WATCHDOG:-150}

if command -v wslpath >/dev/null 2>&1; then
	host_path() { wslpath -w "$1"; }
	separator='\'
else
	host_path() { printf '%s\n' "$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"; }
	separator=/
fi

# The emulator processes running now (WSL: Windows pcsx-redux.main ids).
emulators() {
	if command -v tasklist.exe >/dev/null 2>&1; then
		tasklist.exe /FI "IMAGENAME eq pcsx-redux.main" /FO CSV /NH </dev/null 2>/dev/null |
			tr -d '\r"' | cut -d, -f2 | grep -E '^[0-9]+$'
	fi
}

close_new_emulators() {
	if command -v tasklist.exe >/dev/null 2>&1; then
		for id in $(emulators); do
			case "$1" in *" $id "*) ;; *) taskkill.exe /F /PID "$id" </dev/null >/dev/null 2>&1 ;; esac
		done
	else
		kill "$2" 2>/dev/null
	fi
}

shared=$(host_path tools/emulator.lua)
while read -r script dir cue card argument; do
	if [ "$card" != - ]; then
		if ! grep -q BASCUS-94221 "$card" 2>/dev/null; then
			echo "$script: $dir skipped: $card holds no save"
			continue
		fi
		cp "$card" "$dir/memcard1.mcd" || exit 1
	fi
	win=$(host_path "$dir")
	iso=$(host_path "$cue")
	lua=$(host_path "tools/$script.lua")
	before=" $(emulators | tr '\n' ' ') "
	echo "$script: $dir started at $(date +%T)"
	# From the run's directory: the emulator writes its crash reporter's files
	# into its working directory.
	(cd "$dir" && exec "$PCSX_REDUX" -portable "$win" -iso "$iso" \
		-memcard1 "$win${separator}memcard1.mcd" -memcard2 "$win${separator}memcard2.mcd" \
		-interpreter -debugger -run -logfile "$win${separator}emulator.log" \
		-dofile "$shared" -dofile "$lua" -exec "${script}_run([[$win]], '$argument')" </dev/null) &
	pid=$!
	# The script's heartbeat adds a line to log.txt every 1800 frames.
	size=none
	changed=$(date +%s)
	while kill -0 "$pid" 2>/dev/null; do
		sleep 2
		now=none
		[ -f "$dir/log.txt" ] && now=$(wc -c <"$dir/log.txt")
		if [ "$now" != "$size" ]; then
			size=$now
			changed=$(date +%s)
		elif [ $(($(date +%s) - changed)) -gt "$watchdog" ]; then
			echo "$script: $dir: log.txt unchanged for $watchdog s; closing the emulator"
			close_new_emulators "$before" "$pid"
		fi
	done
	wait "$pid"
	echo "$script: $dir ended at $(date +%T) (exit $?)"
done <"$runs"
