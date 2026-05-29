/* ###
 * Full Lemonade Tycoon 2 cleanup pipeline for one current Ghidra program.
 */
//@category Lemonade Tycoon 2

import com.lemonadetycoon.ghidra.AnalysisScriptSupport;

public class FullAnalysis extends AnalysisScriptSupport {

	@Override
	public void run() throws Exception {
		if (currentProgram == null) {
			println("FullAnalysis: no current program");
			return;
		}
		println("FullAnalysis: " + currentProgram.getName() +
			" initial FUN_* count = " + countDefaultFunctions());
		runStage("Borland demangler", "BorlandDemangler.java");
		runStage("Windows types", "ApplyWindowsTypes.java");
		runStage("Anti-disassembly strip", "StripAntiDisassembly.java");
		runStage("CRT resolver", "ResolveCrtWrappers.java");
		runStage("IAT thunk resolver", "ResolveImportThunks.java");
		System.setProperty("lemonade.populate.skipAuto", "true");
		try {
			runStage("Findings population", "PopulateFindings.java");
		}
		finally {
			System.clearProperty("lemonade.populate.skipAuto");
		}
		println("FullAnalysis: " + currentProgram.getName() +
			" final FUN_* count = " + countDefaultFunctions());
	}

	private void runStage(String label, String script) throws Exception {
		int before = countDefaultFunctions();
		println("FullAnalysis: starting " + label + " (FUN_*=" + before + ")");
		runScript(script);
		int after = countDefaultFunctions();
		println("FullAnalysis: finished " + label + " (FUN_* " +
			before + " -> " + after + ")");
	}

}
