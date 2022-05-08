{
	pname = "gotktrix";
	version = "0.0.1-tip";
	vendorSha256 = "08a03fsrvncfksaz9726mi51ba20g2q19bxxw06brd0bk0v354lr";

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
