# Flake checks for application-level validation (Go).
{
  pkgs,
  makework,
  src ? pkgs.lib.cleanSource ./..,
}:
{
  build = makework;

  lint =
    pkgs.runCommand "makework-lint"
      {
        nativeBuildInputs = [
          pkgs.go
          pkgs.golangci-lint
        ];
        inherit src;
      }
      ''
        export HOME=$TMPDIR
        export GOFLAGS="-mod=mod"
        cd $src
        golangci-lint run --timeout 5m ./...
        touch $out
      '';

  tests =
    pkgs.runCommand "makework-tests"
      {
        nativeBuildInputs = [
          pkgs.go
          pkgs.git
        ];
        inherit src;
      }
      ''
        export HOME=$TMPDIR
        export GIT_CONFIG_GLOBAL=$TMPDIR/.gitconfig
        git config --global user.email "test@test.com"
        git config --global user.name "Test"
        cd $src
        go test -count=1 ./...
        touch $out
      '';

  formatting =
    pkgs.runCommand "makework-fmt-check"
      {
        nativeBuildInputs = [ pkgs.go ];
        inherit src;
      }
      ''
        cd $src
        unformatted=$(gofmt -l .)
        if [ -n "$unformatted" ]; then
          echo "The following files need gofmt:"
          echo "$unformatted"
          exit 1
        fi
        touch $out
      '';
}
