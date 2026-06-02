# Atlas config for authoring versioned migrations from the host
# (atlas migrate diff / lint / hash). The compose `migrate` service applies them.
#
#   atlas migrate diff <name> --env local
#   atlas migrate hash --env local
#
# `dev` is an ephemeral DB Atlas spins up to compute diffs; it requires Docker.

env "local" {
  migration {
    dir = "file://db/migrations"
  }
  dev = "docker://postgres/18/dev"
}
