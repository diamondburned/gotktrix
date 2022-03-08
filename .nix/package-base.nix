{
	pname = "gotktrix";
	version = "0.0.1-tip";
	vendorSha256 = "0mk590dssawdlgzl381vmzn7k00jahz6xq783al5rlqd9a0lmgq6";

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
