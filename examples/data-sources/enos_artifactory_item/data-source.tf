data "enos_artifactory_item" "vault" {
  username = "some-user@your-org.com"
  token    = "1234abcd"

  host = "https://artifactory.example.org/artifactory"
  repo = "myappartifacts/*"
  path = "a/nested/path/in/my/repo/*"
  name = "*.zip"

  # Any property tags on the artifact
  properties = {
    "EDITION"         = "ent"
    "GOARCH"          = "amd64"
    "GOOS"            = "linux"
    "artifactType"    = "package"
    "productRevision" = "f45845666b4e552bfc8ca775834a3ef6fc097fe0"
    "productVersion"  = "1.7.0"
  }
}
