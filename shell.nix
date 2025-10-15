{pkgs ? import <nixpkgs> {}}:

pkgs.mkShell {
	nativeBuildInputs = with pkgs; [
		#apache-jena-fuseki
		#lua
		#luarocks

		go

		gopls
		delve
		go-tools
	];
}
