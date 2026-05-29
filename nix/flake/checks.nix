{ self, ... }:

{
  perSystem =
    { config, pkgs, ... }:
    let
      shared = import ./lib.nix { inherit pkgs self; };
    in
    {
      checks = {
        default = config.packages.lsx-server;
        inherit (config.packages)
          keyvis-search
          lt2findings
          lt2install
          lt2keygen
          lt2normalize
          rbdecompress
          shortv3derive
          ;

        root-node = pkgs.callPackage ../checks/root-node.nix {
          inherit self;
          inherit (shared) version;
        };
        static-unpacking = pkgs.callPackage ../checks/static-unpacking.nix {
          inherit self;
          inherit (shared) pythonAnalysisEnv version;
        };
      };
    };
}
