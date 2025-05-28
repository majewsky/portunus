{ pkgs ? import <nixpkgs> { } }:

with pkgs;

mkShell {
  nativeBuildInputs = [
    go

    # keep this line if you use bash
    bashInteractive
  ];

  buildInputs = [
    libxcrypt
  ];
}
