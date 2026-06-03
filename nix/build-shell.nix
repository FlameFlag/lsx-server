{
  pkgs ? import <nixpkgs> { },
}:

let
  golangciLintPackage = import (pkgs.path + "/pkgs/by-name/go/golangci-lint/package.nix");
  golangciLintArgs = builtins.functionArgs golangciLintPackage;
  golangciLintOverrides =
    if builtins.hasAttr "buildGo126Module" golangciLintArgs then
      { buildGo126Module = pkgs.buildGo126Module; }
    else if builtins.hasAttr "buildGo125Module" golangciLintArgs then
      { buildGo125Module = pkgs.buildGo126Module; }
    else
      { };
  golangciLint = pkgs.callPackage (
    pkgs.path + "/pkgs/by-name/go/golangci-lint/package.nix"
  ) golangciLintOverrides;
in
pkgs.mkShell {
  packages = [
    pkgs.bash
    pkgs.coreutils
    pkgs.findutils
    pkgs.git
    golangciLint
    pkgs.go_1_26
    pkgs.gradle
    pkgs.jdk21_headless
    pkgs.nodejs_24
  ];
}
