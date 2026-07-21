# dev environment composition

module "network" {
  source = "../../modules/network"
}

module "security" {
  source = "../../modules/security"
}

module "iam" {
  source = "../../modules/iam"
}

module "s3" {
  source = "../../modules/s3"
}

module "cloudfront" {
  source = "../../modules/cloudfront"
}

module "alb" {
  source = "../../modules/alb"
}

module "compute" {
  source = "../../modules/compute"
}

module "rds" {
  source = "../../modules/rds"
}

module "observability" {
  source = "../../modules/observability"
}
