{ systemChannel ? <nixpkgs> }:

let systemPkgs = import systemChannel {
		overlays = [ (import ./overlay.nix) ];
	};
	lib = systemPkgs.lib;

	src  = import ./src.nix;
	pkgs = import src.nixpkgs {
		overlays = [ (import ./overlay.nix) ];
	};

in
	if (lib.versionAtLeast systemPkgs.gtk4.version "4.2.0")
	# Prefer the system's Nixpkgs if it's new enough.
	then systemPkgs
	# Else, fetch our own.
	else pkgs
