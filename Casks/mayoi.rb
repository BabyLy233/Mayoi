cask "mayoi" do
  arch arm: "arm64", intel: "amd64"

  version "0.13.1"
  sha256 arm:   "c249f8648ed816a3c0c6d48e00ee9c0a83139508babbf2fbb92039588193d0a4",
         intel: "81fd5cec6855b161c9049c5af84bfd5067e638abd62d10d98addd0457955120c"

  url "https://github.com/sorrycc/Mayoi/releases/download/v#{version}/mayoi-#{version}-#{arch}.dmg"
  name "Mayoi"
  desc "Small, fast code editor"
  homepage "https://github.com/sorrycc/Mayoi"

  auto_updates true
  depends_on macos: :ventura

  app "Mayoi.app"
  command_wrapper "mayoi", content: <<~SH
    #!/bin/sh
    # Managed by Homebrew; use brew uninstall --cask mayoi to remove it.
    app=#{(appdir/"Mayoi.app/Contents/MacOS/Mayoi").to_s.shellescape}
    [ -x "$app" ] || { echo "mayoi: Mayoi.app is not found" >&2; exit 1; }
    MAYOI_CLI=1 exec "$app" "$@"
  SH
end
