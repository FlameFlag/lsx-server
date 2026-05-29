{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
  packages = [
    pkgs.bash
    pkgs.gnumake
    pkgs.clang
    pkgs.python311
    pkgs.uv
  ];
}
