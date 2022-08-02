opts@{ action, ... }:

let src = import ./src.nix;

	pkgs = import "${src.gotk4-nix}/pkgs.nix" {
		overlays = [
			(import ./overlay.nix)
		];
	};

in import "${src.gotk4-nix}/${action}.nix" {
	base = import ./base.nix;
	pkgs = pkgs;
} // opts
