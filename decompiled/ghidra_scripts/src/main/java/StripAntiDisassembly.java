/* ###
 * Strip obvious Lemonade Tycoon 2 protector anti-disassembly edges from the Ghidra project.
 *
 * This patches only the Ghidra database, not the binaries on disk.  It replaces
 * direct branches/calls that point to unmapped memory or into the middle of an
 * existing instruction with NOPs, preserving original bytes in JSONL events and in
 * plate comments.
 */
//@category Lemonade Tycoon 2

import ghidra.app.cmd.disassemble.DisassembleCommand;
import ghidra.program.model.address.Address;
import ghidra.program.model.address.AddressSet;
import ghidra.program.model.listing.Function;
import ghidra.program.model.listing.Instruction;
import ghidra.program.model.listing.InstructionIterator;
import ghidra.program.model.symbol.SourceType;
import com.lemonadetycoon.ghidra.AnalysisScriptSupport;
import com.lemonadetycoon.ghidra.analysis.BadFlow;
import com.google.gson.JsonObject;
import java.io.File;
import java.util.Arrays;

public class StripAntiDisassembly extends AnalysisScriptSupport {

	private static final String DEFAULT_LOG = "decompiled/analysis/anti_disasm_patches.jsonl";
	private static final String EVENT_SCHEMA = "lt2.analysis_event.v1";
	private static final String STAGE = "anti_disassembly_strip";
	private static final int MAX_PASSES = 8;

	@Override
	public void run() throws Exception {
		if (currentProgram == null) {
			println("StripAntiDisassembly: no current program");
			return;
		}

			File log = resolveProjectLogFile("lemonade.antidisasm.log",
				"lt2.antidisasm.log", DEFAULT_LOG);
			ensureFreshLogFile(log);
		int considered = 0;
		int patched = 0;
		int passes = 0;

		int transaction = currentProgram.startTransaction("Lemonade Tycoon 2 strip anti-disassembly");
		try {
			for (int pass = 1; pass <= MAX_PASSES && !monitor.isCancelled(); pass++) {
				passes = pass;
				int passPatched = 0;
				InstructionIterator instructions =
					currentProgram.getListing().getInstructions(true);
				while (instructions.hasNext() && !monitor.isCancelled()) {
					Instruction instruction = instructions.next();
					considered++;
					BadFlow badFlow = directBadFlow(instruction);
					if (badFlow == null) {
						continue;
					}
					if (patchInstruction(instruction, badFlow, log)) {
						patched++;
						passPatched++;
					}
				}
				if (passPatched == 0) {
					break;
				}
				println("StripAntiDisassembly: pass " + passes +
					" patched " + passPatched + " edge(s)");
			}
		}
		finally {
			currentProgram.endTransaction(transaction, true);
		}

		println("StripAntiDisassembly: considered " + considered +
			" instruction(s) over " + passes + " pass(es), patched " + patched +
			" anti-disassembly edge(s)");
	}

	private boolean patchInstruction(Instruction instruction, BadFlow badFlow, File log)
			throws Exception {
		Address start = instruction.getMinAddress();
		int length = instruction.getLength();
		if (length <= 0) {
			return false;
		}
		byte[] original = instruction.getBytes();
		boolean alreadyNops = true;
		for (byte b : original) {
			if ((b & 0xff) != 0x90) {
				alreadyNops = false;
				break;
			}
		}
		if (alreadyNops) {
			return false;
		}

		byte[] nops = new byte[length];
		Arrays.fill(nops, (byte)0x90);

		Address end = start.add(length - 1);
		clearListing(start, end);
		setBytes(start, nops);
		AddressSet changed = new AddressSet(start, end);
		new DisassembleCommand(start, changed, true).applyTo(currentProgram, monitor);

		setPlateComment(start,
			"Lemonade Tycoon 2 stripped anti-disassembly edge\n" +
			"Original bytes: " + bytesToHex(original) + "\n" +
			"Target: " + badFlow.target() + "\n" +
			"Reason: " + badFlow.reason() + "\n" +
			"Project-only patch; original binary on disk is unchanged.");
		appendLog(log, start, original, badFlow);
		markContainingFunction(start, badFlow);
		return true;
	}

	private void markContainingFunction(Address address, BadFlow badFlow) {
		Function function = getFunctionContaining(address);
		if (function == null || !function.getName().startsWith("FUN_")) {
			return;
		}
		String newName = "AutoReview_BadFlow_" + function.getEntryPoint();
		try {
			function.setName(newName, SourceType.ANALYSIS);
			setPlateComment(function.getEntryPoint(),
				"Lemonade Tycoon 2 auto-review: stripped anti-disassembly flow inside this function\n" +
				"Patched edge at: " + address + "\n" +
				"Target: " + badFlow.target() + "\n" +
				"Reason: " + badFlow.reason());
		}
		catch (Exception e) {
			println("StripAntiDisassembly: could not mark " + function.getEntryPoint() +
				": " + e.getMessage());
		}
	}

	private void appendLog(File file, Address address, byte[] original, BadFlow badFlow)
			throws Exception {
		JsonObject event = new JsonObject();
		event.addProperty("schema", EVENT_SCHEMA);
		event.addProperty("stage", STAGE);
		event.addProperty("program", currentProgram.getName());
		event.addProperty("executable_md5", currentProgram.getExecutableMD5());
		event.addProperty("address", address.toString());
		event.addProperty("action", "patched_instruction");
		event.addProperty("category", "anti_disassembly_patch");
		event.addProperty("decision", "accepted");
		event.addProperty("original_bytes", bytesToHex(original));
		event.addProperty("patched_bytes", repeatedByteHex(original.length, "90"));
		event.addProperty("target", badFlow.target().toString());
		event.addProperty("evidence", badFlow.reason());
		appendJsonLine(file, event);
	}
}
