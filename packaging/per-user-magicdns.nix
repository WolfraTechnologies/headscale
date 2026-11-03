{ headscale, fetchgit }:

headscale.overrideAttrs (old: {
  version = "0.28.0-per-user-magicdns";
  src =
    (fetchgit {
      # Use the reviewed fork URL after publishing. The local file URL is used
      # for the disposable build verification documented below.
      url = "file:///home/kgadberry/projects/Wolfra_IT/.scratch/headscale-fork";
      rev = "147666ce48cdeeda69e51f96de7f0fc5db979e41";
      hash = "sha256-EYIdCiVQqByt3oxQy11bCkAIPrPQJi/RJcv3W+xbkTg=";
    })
    // {
      tag = "prototype/per-user-magicdns";
    };
  # No Go module dependency changed, so retain upstream's vendor hash.
  vendorHash = "sha256-jkeB9XUTEGt58fPOMpE4/e3+JQoMQTgf0RlthVBmfG0=";
})
