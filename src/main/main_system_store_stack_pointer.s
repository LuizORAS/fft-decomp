/* Store sp in the word a0 points to: main saves the game loop's base stack
 * this way at boot, and main_restore_game_loop_stack_pointer loads it back
 * for main_system_reset_game. Assembly because reading $sp is a
 * stack-management ABI boundary that C cannot express; like its neighbor and
 * wldcore_system_switch_to_stack, it copies a0 to t0 first and leaves the
 * jr ra delay slot empty.
 *
 * SCUS_942.21 0x80011bb0..0x80011bbf (file 0x23b0..0x23bf).
 */
	.set	noreorder
	.set	noat
	.text
	.align	2
	.globl	main_system_store_stack_pointer
	.ent	main_system_store_stack_pointer
main_system_store_stack_pointer:
	addu	$t0, $a0, $zero
	sw	$sp, 0($t0)
	jr	$ra
	nop
	.end	main_system_store_stack_pointer
	.size	main_system_store_stack_pointer, . - main_system_store_stack_pointer
