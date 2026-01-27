resource "yandex_iam_service_account" "bucket" {
    name = "bucket-sa"
}

resource "yandex_resourcemanager_folder_iam_member" "storage_editor" {
  folder_id = "b1glbk1npdsuscq293co"
  role      = "storage.editor"
  member    = "serviceAccount:${yandex_iam_service_account.bucket.id}"
}

resource "yandex_iam_service_account_static_access_key" "this" {
    service_account_id = yandex_iam_service_account.bucket.id
    description = "static access key for object storage"
}

resource "yandex_storage_bucket" "this" {
    bucket = "pcap"
    access_key = yandex_iam_service_account_static_access_key.this.access_key
    secret_key = yandex_iam_service_account_static_access_key.this.secret_key
    max_size = 10485760
    default_storage_class = "standard"

    depends_on = [ yandex_resourcemanager_folder_iam_member.storage_editor ]
}

resource "yandex_lockbox_secret" "s3_keys" {
  name = "s3-static-keys"
}

resource "yandex_lockbox_secret_version" "v1" {
  secret_id = yandex_lockbox_secret.s3_keys.id

  entries {
    key = "access_key"
    text_value = yandex_iam_service_account_static_access_key.this.access_key
  }

  entries {
    key = "secret_key"
    text_value = yandex_iam_service_account_static_access_key.this.secret_key
  }
}

resource "kubernetes_secret" "yc_s3_keys" {
  metadata {
    name = "yc-s3-keys"
    namespace = "default"
  }

  type = "Opaque"

  data = {
    "access-key" = yandex_iam_service_account_static_access_key.this.access_key
    "secret-key" = yandex_iam_service_account_static_access_key.this.secret_key
  }

  depends_on = [yandex_iam_service_account_static_access_key.this]
}