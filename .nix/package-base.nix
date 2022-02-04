{
	pname = "gotktrix";
	version = "0.0.1-tip";
	vendorSha256 = "1r4kcr4ipa3wm1yflp624saia8c1n1jdm5681ag2fm4fylrgdvvq";

	buildInputs = buildPkgs: with buildPkgs; [
		gtk4
		glib
		graphene
		gdk-pixbuf
		gobjectIntrospection
		hicolor-icon-theme

		# Optional
		sound-theme-freedesktop
		libcanberra-gtk3
	];

	nativeBuildInputs = pkgs: with pkgs; [
		pkgconfig
	];
}
