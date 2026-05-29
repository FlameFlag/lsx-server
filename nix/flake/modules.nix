{ self, ... }:

{
  flake = {
    nixosModules =
      let
        lsx-server = import ../modules/nixos.nix { defaultPackage = self.packages; };
      in
      {
        inherit lsx-server;
        default = lsx-server;
      };

    homeManagerModules =
      let
        lsx-server = import ../modules/home-manager.nix { defaultPackage = self.packages; };
      in
      {
        inherit lsx-server;
        default = lsx-server;
      };

    homeModules = self.homeManagerModules;

    darwinModules =
      let
        lsx-server = import ../modules/nix-darwin.nix { defaultPackage = self.packages; };
      in
      {
        inherit lsx-server;
        default = lsx-server;
      };
  };
}
