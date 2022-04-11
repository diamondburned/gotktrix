{
	pname = "gotktrix";
	version = "0.0.1-tip";
	vendorSha256 = "1yaajqcghxc5wxs887pnnrigj6m1gaplk5kd24gkwsgsiddq63f4";

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
