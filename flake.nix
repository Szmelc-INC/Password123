{
  description = "Password123: zero-knowledge cryptography lab (p123 CLI/TUI)";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAll = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
      version = "0.1.0";
    in {
      packages = forAll (pkgs: rec {
        p123 = pkgs.buildGoModule {
          pname = "p123";
          inherit version;
          src = self;
          subPackages = [ "cmd/p123" ];
          vendorHash = "sha256-7g2/dIpiqkJxwB8euuQfjV9Q8SNBCE7FUmOA9kkFin4=";
          env.CGO_ENABLED = "0";
          ldflags = [ "-s" "-w" "-X github.com/Szmelc-INC/Password123/internal/cli.Version=v${version}" ];
          nativeBuildInputs = [ pkgs.installShellFiles ];
          # Repository-level tests need the checkout (vectors, payloads); CI runs them.
          doCheck = false;
          postInstall = ''
            installShellCompletion --cmd p123 \
              --bash completions/p123.bash \
              --zsh completions/_p123 \
              --fish completions/p123.fish
            install -Dm644 LICENSE $out/share/licenses/p123/LICENSE
            install -Dm644 README.md ARCHITECTURE.md CONTRIBUTING.md SECURITY.md -t $out/share/doc/p123
          '';
          meta = {
            description = "Zero-knowledge cryptography lab CLI/TUI";
            homepage = "https://github.com/Szmelc-INC/Password123";
            license = pkgs.lib.licenses.mit;
            mainProgram = "p123";
          };
        };
        default = p123;
      });

      apps = forAll (pkgs: {
        default = {
          type = "app";
          program = "${self.packages.${pkgs.stdenv.hostPlatform.system}.p123}/bin/p123";
        };
      });

      devShells = forAll (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [ go gopls age sops gitleaks nodejs_22 ];
        };
      });
    };
}
