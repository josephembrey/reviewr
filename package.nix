{
  buildGoModule,
  git,
  lib,
  makeWrapper,
}:
buildGoModule {
  pname = "reviewr";
  version = "0.1.0";

  src = lib.cleanSource ./.;
  vendorHash = "sha256-/am1skPJNEmyT1VGqzsWZetuNUl/958n1IurcNyQZ7M=";

  subPackages = ["cmd/reviewr"];
  nativeBuildInputs = [makeWrapper];
  doCheck = false;

  postInstall = ''
    wrapProgram $out/bin/reviewr --prefix PATH : ${lib.makeBinPath [git]}
  '';
  ldflags = [
    "-s"
    "-w"
  ];

  meta = {
    description = "Terminal application for reviewing repository changes";
    homepage = "https://github.com/josephembrey/reviewr";
    license = lib.licenses.mit;
    mainProgram = "reviewr";
  };
}
