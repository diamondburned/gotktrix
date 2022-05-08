opts@{ action, ... }:

let src = import ./src.nix;

	pkgs = import "${src.gotk4-nix}/pkgs.nix" {
		overlays = [
			(self: super: {
				gtk4 = super.gtk4.overrideAttrs(old: {
					# https://github.com/diamondburned/gotktrix/issues/33
					version = "4.6.3-git";
					src = super.fetchFromGitLab {
						domain = "gitlab.gnome.org";
						owner  = "GNOME";
						repo   = "gtk";
						rev    = "2441409b34736a149ff2839be28a5be0b3067bd7";
						sha256 = "01cxv4sx5r0k1vzq3y3cxkafi7xhf10zpnnyxs63143kr5kpn17s";
					};
					mesonFlags = old.mesonFlags ++ [
						"-Dgtk_doc=false"
						"-Dtracker=disabled"
					];
					outputs = [ "out" "dev" ];
				});
			})
		];
	};

in import "${src.gotk4-nix}/${action}.nix" {
	base = import ./base.nix;
	pkgs = pkgs;
} // opts
