{
  description = "Lemonade Tycoon 2 LSX compatibility server";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/master";

  outputs =
    inputs:
    let
      systems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-darwin"
        "x86_64-linux"
      ];

      forAllSystems = inputs.nixpkgs.lib.genAttrs systems;
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = import inputs.nixpkgs { inherit system; };
          version = inputs.self.shortRev or inputs.self.dirtyShortRev or "dev";

          shikiVendor = pkgs.stdenvNoCC.mkDerivation {
            pname = "lsx-server-shiki-vendor";
            inherit version;

            nativeBuildInputs = [
              pkgs.cacert
              pkgs.go_1_26
            ];

            outputHashAlgo = "sha256";
            outputHashMode = "recursive";
            outputHash = "sha256-O8uzZQdwoYAV8U4aZz/qFezDEiVQKurbj/P7A74+QMA=";

            dontUnpack = true;

            buildPhase = ''
              export HOME="$TMPDIR"
              export GOCACHE="$TMPDIR/go-cache"
              mkdir -p assets tools
              cp ${./tools/download_findings_shiki.go} tools/download_findings_shiki.go
              cd assets
              go run ../tools/download_findings_shiki.go
            '';

            installPhase = ''
              mkdir -p "$out"
              cp -R project/findings/vendor/shiki/unpkg "$out/"
            '';
          };
        in
        {
          default = pkgs.buildGo126Module {
            pname = "lsx-server";
            inherit version;

            src = inputs.self;
            vendorHash = "sha256-1KjBacFMKNu3lLA3LC/NB7YmwB+eubnGrCqsYGDK34Q=";

            env.CGO_ENABLED = 0;
            subPackages = [ "." ];

            postConfigure = ''
              mkdir -p assets/project/findings/vendor/shiki
              cp -R ${shikiVendor}/unpkg assets/project/findings/vendor/shiki/
            '';

            ldflags = [
              "-s"
              "-w"
              "-X main.version=${version}"
            ];
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
    };
}
