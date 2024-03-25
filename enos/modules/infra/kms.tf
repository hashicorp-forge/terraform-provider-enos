resource "aws_kms_key" "enos_key" {
  description             = "Enos Key"
  deletion_window_in_days = 7
}

resource "aws_kms_alias" "enos_key_alias" {
  name          = "alias/enos_key-${random_string.cluster_id.result}"
  target_key_id = aws_kms_key.enos_key.key_id
}
