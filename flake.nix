{
  description = "Lemonade Tycoon 2 LSX compatibility server";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
  };

  outputs =
    inputs:
    inputs.flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [
        ./nix/flake/packages.nix
        ./nix/flake/checks.nix
        ./nix/flake/dev-shells.nix
        ./nix/flake/modules.nix
      ];

      systems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-darwin"
        "x86_64-linux"
      ];
    };
}
