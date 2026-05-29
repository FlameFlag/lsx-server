{
  fetchurl,
  stdenvNoCC,
  unzip,
}:

let
  version = "1.24.0";
in
stdenvNoCC.mkDerivation {
  pname = "PSScriptAnalyzer";
  inherit version;

  src = fetchurl {
    url = "https://www.powershellgallery.com/api/v2/package/PSScriptAnalyzer/${version}";
    hash = "sha256-6GyX1EuxvIod4151O4XqHZOPb5+IFjmhgVB+B5vKRVY=";
  };

  nativeBuildInputs = [ unzip ];
  unpackPhase = ''
    unzip "$src"
  '';

  installPhase = ''
    mkdir -p "$out/share/powershell/Modules/PSScriptAnalyzer/${version}"
    cp -R . "$out/share/powershell/Modules/PSScriptAnalyzer/${version}/"
  '';
}
