group default {
  targets = ["psst-bin"]
}

variable DEST {
  default = "./_out/"
}

target psst {
  target = "psst"
  tags = ["docker.io/milas/psst"]
  platforms = ["linux/amd64", "linux/arm64"]
}

target psst-bin {
  target = "psst-bin"
  output = ["type=local,dest=${DEST}"]
  platforms = ["local"]
}
