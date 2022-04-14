{
	pname = "gotktrix";
	version = "0.0.1-tip";
	vendorSha256 = "08a03fsrvncfksaz9726mi51ba20g2q19bxxw06brd0bk0v354lr";

	buildInputs = buildPkgs: with buildPkgs; [
		gtk4
		glib
		graphene
		gdk-pixbuf
		gobject-introspection
		hicolor-icon-theme

		# Optional
		sound-theme-freedesktop
		libcanberra-gtk3
	];

	nativeBuildInputs = pkgs: with pkgs; [
		pkgconfig
	];
}
