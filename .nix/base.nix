{
	pname = "gotktrix";
	version = "0.0.1-tip";
	vendorSha256 = "0cmandfdkczpppmf1kdxliw2b164a48vh9iqb46vizab69ynv7j7";

	src = ../.;

	buildInputs = buildPkgs: with buildPkgs; [
		# Optional
		sound-theme-freedesktop
		libcanberra-gtk3
	];

	files = {
		desktop = {
			name = "com.github.diamondburned.gotktrix.desktop";
			path = ./com.github.diamondburned.gotktrix.desktop;
		};
		logo = {
			name = "gotktrix.png";
			path = ../.github/logo-256.png;
		};
	};
}
