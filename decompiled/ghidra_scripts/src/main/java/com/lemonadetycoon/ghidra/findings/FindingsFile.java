package com.lemonadetycoon.ghidra.findings;

import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.util.ArrayList;
import java.util.List;

public final class FindingsFile {
	private FindingsFile() {
	}

	public static List<Finding> load(File file) throws Exception {
		List<Finding> findings = new ArrayList<Finding>();
		List<String> lines = Files.readAllLines(file.toPath(), StandardCharsets.UTF_8);
		Finding current = null;
		for (int lineNo = 0; lineNo < lines.size(); lineNo++) {
			String line = lines.get(lineNo).trim();
			if (line.isEmpty() || line.startsWith("#") || line.startsWith(";")) {
				continue;
			}
			if (line.startsWith("[")) {
				if (!line.endsWith("]")) {
					throw new Exception("findings file line " + (lineNo + 1) +
						" has a malformed section header");
				}
				if (current != null) {
					current.validate(lineNo + 1);
					findings.add(current);
				}
				String id = line.substring(1, line.length() - 1).trim();
				if (id.isEmpty()) {
					throw new Exception("findings file line " + (lineNo + 1) +
						" has an empty finding id");
				}
				current = new Finding();
				current.id = id;
				continue;
			}
			if (current == null) {
				throw new Exception("findings file line " + (lineNo + 1) +
					" has a key/value before the first section");
			}
			int equals = line.indexOf('=');
			if (equals < 0) {
				throw new Exception("findings file line " + (lineNo + 1) +
					" expected key = value");
			}
			String key = line.substring(0, equals).trim();
			String value = line.substring(equals + 1).trim();
			current.set(key, value, lineNo + 1);
		}
		if (current != null) {
			current.validate(lines.size());
			findings.add(current);
		}
		if (findings.isEmpty()) {
			throw new Exception("findings file has no sections");
		}
		return findings;
	}
}
