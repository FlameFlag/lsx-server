{ python311 }:

python311.withPackages (ps: [
  ps.capstone
  ps.construct
  ps.pefile
])
