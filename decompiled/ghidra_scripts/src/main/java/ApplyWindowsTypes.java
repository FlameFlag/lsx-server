/* ###
 * Lemonade Tycoon 2 Windows type helper.
 *
 * Adds a compact Win32 typedef set to the program and applies it only where
 * existing parameter names already make the intended type clear.
 */
//@category Lemonade Tycoon 2

import ghidra.program.model.data.CategoryPath;
import ghidra.program.model.data.CharDataType;
import ghidra.program.model.data.DataType;
import ghidra.program.model.data.DataTypeConflictHandler;
import ghidra.program.model.data.DWordDataType;
import ghidra.program.model.data.PointerDataType;
import ghidra.program.model.data.TypedefDataType;
import ghidra.program.model.data.VoidDataType;
import ghidra.program.model.listing.Function;
import ghidra.program.model.listing.FunctionIterator;
import ghidra.program.model.listing.Parameter;
import ghidra.program.model.symbol.SourceType;
import com.lemonadetycoon.ghidra.AnalysisScriptSupport;
import java.util.Locale;

public class ApplyWindowsTypes extends AnalysisScriptSupport {

	@Override
	public void run() throws Exception {
		if (currentProgram == null) {
			println("ApplyWindowsTypes: no current program");
			return;
		}
		WinTypes types = installMinimalTypes();
		int changedParams = 0;
		int changedReturns = 0;

		int transaction = currentProgram.startTransaction("Lemonade Tycoon 2 apply Windows types");
		try {
			FunctionIterator functions =
				currentProgram.getFunctionManager().getFunctions(true);
			while (functions.hasNext() && !monitor.isCancelled()) {
				Function function = functions.next();
				Parameter[] params = function.getParameters();
				for (int i = 0; i < params.length; i++) {
					DataType replacement = inferParameterType(params[i], types);
					if (replacement == null) {
						continue;
					}
					try {
						params[i].setDataType(replacement, SourceType.ANALYSIS);
						changedParams++;
					}
					catch (Exception e) {
						println("Could not type " + function.getName() + "::" +
							params[i].getName() + ": " + e.getMessage());
					}
				}
				DataType returnType = inferReturnType(function, types);
				if (returnType != null) {
					try {
						function.setReturnType(returnType, SourceType.ANALYSIS);
						changedReturns++;
					}
					catch (Exception e) {
						println("Could not type return for " + function.getName() +
							": " + e.getMessage());
					}
				}
			}
		}
		finally {
			currentProgram.endTransaction(transaction, true);
		}

		println("ApplyWindowsTypes: installed minimal Win32 typedefs, typed " +
			changedParams + " parameter(s), " + changedReturns + " return(s)");
		println("ApplyWindowsTypes: Ghidra SDK archives are normally under " +
			"<ghidra>/Ghidra/Features/Base/data/typeinfo/windows_vs12_32.gdt");
	}

	private WinTypes installMinimalTypes() {
		CategoryPath path = new CategoryPath("/Lemonade Tycoon 2/Win32");
		WinTypes types = new WinTypes();
		types.DWORD = add(new TypedefDataType(path, "DWORD", DWordDataType.dataType));
		types.BOOL = add(new TypedefDataType(path, "BOOL", DWordDataType.dataType));
		types.LPVOID = add(new TypedefDataType(path, "LPVOID",
			new PointerDataType(VoidDataType.dataType, currentProgram.getDataTypeManager())));
		types.HWND = add(new TypedefDataType(path, "HWND", types.LPVOID));
		types.HINSTANCE = add(new TypedefDataType(path, "HINSTANCE", types.LPVOID));
		types.HMODULE = add(new TypedefDataType(path, "HMODULE", types.LPVOID));
		types.HANDLE = add(new TypedefDataType(path, "HANDLE", types.LPVOID));
		DataType charPtr = new PointerDataType(CharDataType.dataType,
			currentProgram.getDataTypeManager());
		types.LPCSTR = add(new TypedefDataType(path, "LPCSTR", charPtr));
		types.LPSTR = add(new TypedefDataType(path, "LPSTR", charPtr));
		return types;
	}

	private DataType add(DataType type) {
		return currentProgram.getDataTypeManager()
			.addDataType(type, DataTypeConflictHandler.REPLACE_HANDLER);
	}

	private DataType inferParameterType(Parameter parameter, WinTypes types) {
		String name = parameter.getName().toLowerCase(Locale.ROOT);
		if (!isFourByteScalar(parameter.getDataType())) {
			return null;
		}
		if (name.equals("hwnd") || name.endsWith("hwnd") || name.contains("window")) {
			return types.HWND;
		}
		if (name.equals("hinstance") || name.equals("hinst")) {
			return types.HINSTANCE;
		}
		if (name.equals("hmodule")) {
			return types.HMODULE;
		}
		if (name.equals("handle") || name.startsWith("hfile") || name.startsWith("hkey")) {
			return types.HANDLE;
		}
		if (name.equals("lpcstr") || name.equals("lpstr") || name.startsWith("psz") ||
			name.contains("caption") || name.contains("title") ||
			name.contains("url") || name.contains("path") || name.contains("file")) {
			return name.equals("lpstr") ? types.LPSTR : types.LPCSTR;
		}
		if (name.equals("dwstyle") || name.startsWith("dw") || name.endsWith("flags")) {
			return types.DWORD;
		}
		return null;
	}

	private DataType inferReturnType(Function function, WinTypes types) {
		String name = function.getName().toLowerCase(Locale.ROOT);
		if (!isFourByteScalar(function.getReturnType())) {
			return null;
		}
		if (name.contains("createwindow") || name.endsWith("_getdlgitem")) {
			return types.HWND;
		}
		if (name.startsWith("iat_kernel32_loadlibrary") || name.contains("loadlibrary")) {
			return types.HMODULE;
		}
		if (name.contains("createfile") || name.contains("openprocess")) {
			return types.HANDLE;
		}
		if (name.startsWith("iat_") && (name.contains("copyfile") ||
			name.contains("deletefile") || name.contains("internet"))) {
			return types.BOOL;
		}
		return null;
	}

	private boolean isFourByteScalar(DataType type) {
		return type != null && type.getLength() == 4 &&
			(type.getName().startsWith("undefined") ||
			 type.getName().equals("int") ||
			 type.getName().equals("uint"));
	}

	private static class WinTypes {
		DataType DWORD;
		DataType BOOL;
		DataType LPVOID;
		DataType HWND;
		DataType HINSTANCE;
		DataType HMODULE;
		DataType HANDLE;
		DataType LPCSTR;
		DataType LPSTR;
	}
}
