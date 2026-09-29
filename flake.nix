{
  # KernelStore — marketplace đa nhà cung cấp.
  # Backend: Go 1.27 + net/http + pgx + PostgreSQL 16. Frontend: Go + templ + HTMX + Tailwind.
  #
  # Cần bật flakes trong /etc/nixos/configuration.nix:
  #   nix.settings.experimental-features = [ "nix-command" "flakes" ];
  #
  #   nix develop                  # vào môi trường dev
  #   nix develop -c ./run.sh      # DB + backend + frontend
  #   nix build                    # build backend  → ./result/bin/kernelstore-api
  #   nix build .#frontend         # build frontend → ./result/bin/kernelstore-web
  #   nix flake update             # nâng phiên bản nixpkgs (ghi lại flake.lock)
  description = "KernelStore — Go 1.27 backend + Go/templ/HTMX frontend";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      # Tự viết forAllSystems thay vì thêm phụ thuộc flake-utils.
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});

      # Dự án yêu cầu Go 1.27 (go.mod: go 1.27, toolchain go1.27.1).
      # nixpkgs có sẵn go_1_27 = 1.27.1 — KHÔNG hạ xuống bản Go mặc định thấp hơn.
      goFor = pkgs: pkgs.go_1_27;
    in
    {
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          name = "kernelstore-dev";
          packages = with pkgs; [
            # Go toolchain
            (goFor pkgs)
            gopls
            delve
            golangci-lint

            # Frontend: templ (sinh code Go từ .templ), Tailwind CLI v3 (khớp tailwind.config.js),
            # air (hot reload, thay trunk serve)
            templ
            tailwindcss_3
            air

            # Database: CLI golang-migrate (backend tự chạy migration khi khởi động),
            # psql client, Docker để chạy Postgres 16 bằng docker compose
            go-migrate
            postgresql_16
            docker-client
            docker-compose

            # Tiện ích cho run.sh và các bộ test_*.sh
            jq
            curl
          ];
          # Không dùng sqlc: truy vấn viết tay bằng pgx (bộ lọc động của products/admin).

          shellHook = ''
            # Dùng đúng Go của flake, không để go tự tải toolchain khác.
            export GOTOOLCHAIN=local
            export GOPATH="''${GOPATH:-$HOME/go}"
            export PATH="$GOPATH/bin:$PATH"

            echo "[OK] KernelStore dev environment (nix develop)"
            echo "  $(go version)"
            echo "  templ $(templ version)"
            echo "  $(tailwindcss --help 2>/dev/null | head -n 2 | tail -n 1 | sed 's/^ *//')"
            echo "  air v${pkgs.air.version}"   # binary air tự báo "(devel)" nên lấy version từ nixpkgs
          '';
        };
      });

      packages = forAllSystems (
        pkgs:
        let
          buildGoModule = pkgs.buildGoModule.override { go = goFor pkgs; };
        in
        rec {
          # Backend API (:5000). Migration SQL được nhúng trong binary.
          # Ảnh upload: đặt UPLOAD_DIR (mặc định ./uploads hoặc ./backend/uploads).
          backend = buildGoModule {
            pname = "kernelstore-backend";
            version = "0.1.0";
            src = ./backend;
            subPackages = [ "cmd/api" ];
            vendorHash = "sha256-QEo21rwkvsmbXgspQdA6k9Ymw5qMStQ3R8VX8KtcB54=";
            env.CGO_ENABLED = 0;
            postInstall = "mv $out/bin/api $out/bin/kernelstore-api";
            meta.mainProgram = "kernelstore-api";
          };

          # Frontend SSR (:8080). Code templ và app.css đã sinh sẵn trong repo.
          frontend = buildGoModule {
            pname = "kernelstore-frontend";
            version = "0.1.0";
            src = ./frontend;
            subPackages = [ "cmd/web" ];
            vendorHash = "sha256-7KZe028i54yv8aXoNqPACcgzn6fl4HtHgA/xHKHhPCg=";
            env.CGO_ENABLED = 0;
            nativeBuildInputs = [ pkgs.makeWrapper ];
            postInstall = ''
              mkdir -p $out/share/kernelstore-frontend
              cp -r static $out/share/kernelstore-frontend/
              mv $out/bin/web $out/bin/kernelstore-web
              wrapProgram $out/bin/kernelstore-web \
                --set-default STATIC_DIR $out/share/kernelstore-frontend/static
            '';
            meta.mainProgram = "kernelstore-web";
          };

          default = backend;
        }
      );

      formatter = forAllSystems (pkgs: pkgs.nixfmt-rfc-style);
    };
}
