{ pkgs ? import ./pkgs.nix {} }:

let src = import ./src.nix;

	shell = import "${src.gotk4}/.nix/shell.nix" {
		inherit pkgs;
	};

	# minitime is a mini-output time wrapper.
	minitime = pkgs.writeShellScriptBin
		"minitime"
		"command time --format $'🕒 -> %es\\n' \"$@\"";


in shell.overrideAttrs (old: {
	buildInputs = old.buildInputs ++ (with pkgs; [
		gtk4.debug
		glib.debug

		libadwaita
	]);

	nativeBuildInputs = old.nativeBuildInputs ++ [
		minitime
	];

	NIX_DEBUG_INFO_DIRS = ''${pkgs.gtk4.debug}/lib/debug:${pkgs.glib.debug}/lib/debug'';

	CGO_ENABLED  = "1";
	CGO_CFLAGS   = "-g2 -O2";
	CGO_CXXFLAGS = "-g2 -O2";
	CGO_FFLAGS   = "-g2 -O2";
	CGO_LDFLAGS  = "-g2 -O2";
})
