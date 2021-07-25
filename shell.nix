{ systemPkgs ? import <nixpkgs> {} }:

let adw_src = systemPkgs.fetchFromGitHub {
		owner = "diamondburned";
		repo  = "gotk4-adwaita";
		rev   = "5420c7113d40b5ed95e25dc684098f911724a23c";
		hash  = "sha256:0q2vccx2q6cmfznn1222bpcly0dpph85ifis3hnnanghqhd3m0sz";
	};

	adw = import "${adw_src}/shell.nix" {
		inherit systemPkgs;
	};

in adw.overrideAttrs(old: {
	buildInputs = old.buildInputs ++ (with adw.pkgs; [
		materia-theme
	]);
})
