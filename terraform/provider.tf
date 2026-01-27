# Объявление провайдера
terraform {
  required_providers {
    kubernetes = {
      source = "hashicorp/kubernetes"
      version = "~> 2.0"
    }
    yandex = {
      source = "yandex-cloud/yandex"
    }
  }
  required_version = ">= 1.00"
}

provider "kubernetes" {
  config_path = "~/.kube/config"
  config_context = "kind-kind"
}

provider "yandex" {
  zone                     = "ru-central1-a"
  folder_id                = "b1glbk1npdsuscq293co"
}