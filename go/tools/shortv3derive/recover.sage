p = 0x1000000000000000E3B
g = 0xF3C7E00A4B58155299
y = 0x9CC50E4D25416464B9
expected_x = 0x70301169DE7C75D66F

x = discrete_log(Mod(y, p), Mod(g, p))

print(f"modulus p: 0x{Integer(p):X}")
print(f"generator g: 0x{Integer(g):X}")
print(f"public y: 0x{Integer(y):X}")
print(f"recovered private exponent x: 0x{Integer(x):X}")
print(f"matches checked-in exponent: {Integer(x) == expected_x}")
print(f"verifies public certificate: {power_mod(g, x, p) == y}")

if Integer(x) != expected_x or power_mod(g, x, p) != y:
    raise SystemExit(1)
