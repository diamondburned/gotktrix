let base = import ./package-base.nix;

in {
	pkgs, lib,

	gotktrixSrc ? ./..,
	suffix ? "",
	buildPkgs ? import ./pkgs.nix {}, # only for overriding
	goPkgs ? buildPkgs,
	wrapGApps ? true,
	vendorSha256 ? base.vendorSha256,
}:

goPkgs.buildGoModule {
	src = gotktrixSrc;
	inherit vendorSha256;
	inherit (base) version;

	pname = base.pname + suffix;

	buildInputs = base.buildInputs buildPkgs;

	nativeBuildInputs = base.nativeBuildInputs pkgs 
		++ (lib.optional wrapGApps [ pkgs.wrapGAppsHook ]);

	subPackages = [ "." ];

	preFixup = ''
		mkdir -p $out/share/icons/hicolor/256x256/apps/ $out/share/applications/
		# Install the desktop file
		cp "${./com.github.diamondburned.gotktrix.desktop}" $out/share/applications/com.github.diamondburned.gotktrix.desktop
		# Install the icon
		cp "${../.github/logo-256.png}" $out/share/icons/hicolor/256x256/apps/gotktrix.png
	'';
}
