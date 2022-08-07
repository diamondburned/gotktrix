{
	pname = "gotktrix";
	version = "0.0.1-tip";
	vendorSha256 = "1cpqm31gkmqycq57sz27p17yhxmdlww0f07wfm036zhbsz1szdcf";

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
