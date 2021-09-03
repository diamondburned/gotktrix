{ systemPkgs ? import <nixpkgs> {} }:

let src = import ./src.nix;

in import src.nixpkgs {
	overlays = [ (import ./overlay.nix) ];
}
