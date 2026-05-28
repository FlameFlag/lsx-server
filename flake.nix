{
  description = "Lemonade Tycoon 2 LSX compatibility server";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    inputs:
    let
      systems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-darwin"
        "x86_64-linux"
      ];

      forAllSystems = inputs.nixpkgs.lib.attrsets.genAttrs systems;
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = import inputs.nixpkgs { inherit system; };
          version = inputs.self.shortRev or inputs.self.dirtyShortRev or "dev";
        in
        {
          default = pkgs.buildGo126Module {
            pname = "lsx-server";
            inherit version;

            src = inputs.self;
            vendorHash = "sha256-IjreT9GBKIabo2hcnVPN/5HMSDr4ieOHNWGaUWB4KXY=";

            env.CGO_ENABLED = 0;
            subPackages = [ "." ];

            ldflags = [
              "-s"
              "-w"
              "-X main.version=${version}"
            ];

            postInstall = ''
              if [ -x "$out/bin/lsx_server_go" ] && [ ! -e "$out/bin/lsx-server" ]; then
                mv "$out/bin/lsx_server_go" "$out/bin/lsx-server"
              fi
            '';

            meta.mainProgram = "lsx-server";
          };
        }
      );

      devShells = forAllSystems (
        system:
        let
          pkgs = import inputs.nixpkgs { inherit system; };
          psScriptAnalyzerVersion = "1.24.0";
          psScriptAnalyzer = pkgs.stdenvNoCC.mkDerivation {
            pname = "PSScriptAnalyzer";
            version = psScriptAnalyzerVersion;

            src = pkgs.fetchurl {
              url = "https://www.powershellgallery.com/api/v2/package/PSScriptAnalyzer/${psScriptAnalyzerVersion}";
              hash = "sha256-6GyX1EuxvIod4151O4XqHZOPb5+IFjmhgVB+B5vKRVY=";
            };

            nativeBuildInputs = [ pkgs.unzip ];
            unpackPhase = ''
              unzip "$src"
            '';

            installPhase = ''
              mkdir -p "$out/share/powershell/Modules/PSScriptAnalyzer/${psScriptAnalyzerVersion}"
              cp -R . "$out/share/powershell/Modules/PSScriptAnalyzer/${psScriptAnalyzerVersion}/"
            '';
          };
        in
        {
          default = pkgs.mkShell {
            packages = [
              pkgs.git
              pkgs.go_1_26
              pkgs.powershell
              pkgs.shellcheck
              pkgs.shfmt
            ];

            shellHook = ''
              export PSModulePath="${psScriptAnalyzer}/share/powershell/Modules''${PSModulePath:+:}$PSModulePath"
            '';
          };
        }
      );

      nixosModules =
        let
          lsx-server = import ./nix/modules/nixos.nix { defaultPackage = inputs.self.packages; };
        in
        {
          inherit lsx-server;
          default = lsx-server;
        };

      homeManagerModules =
        let
          lsx-server = import ./nix/modules/home-manager.nix { defaultPackage = inputs.self.packages; };
        in
        {
          inherit lsx-server;
          default = lsx-server;
        };

      homeModules = inputs.self.homeManagerModules;

      darwinModules =
        let
          lsx-server = import ./nix/modules/nix-darwin.nix { defaultPackage = inputs.self.packages; };
        in
        {
          inherit lsx-server;
          default = lsx-server;
        };
    };
}
