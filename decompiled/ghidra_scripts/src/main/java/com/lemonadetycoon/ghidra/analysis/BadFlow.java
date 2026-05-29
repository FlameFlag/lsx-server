package com.lemonadetycoon.ghidra.analysis;

import ghidra.program.model.address.Address;

public class BadFlow {
	private final Address target;
	private final String reason;

	public BadFlow(Address target, String reason) {
		this.target = target;
		this.reason = reason;
	}

	public Address target() {
		return target;
	}

	public String reason() {
		return reason;
	}
}
