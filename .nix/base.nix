{
	pname = "gotktrix";
	version = "0.0.1-tip";
	vendorSha256 = "1hq18s5q45mfv2c1q6jb2d7msr08df044f7c36yxb6vxsiy5v8dz";

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
