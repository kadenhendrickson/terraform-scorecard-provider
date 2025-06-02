terraform {
  required_providers {
    scorecard = {
      source  = "local/scorecard"
      version = "0.1.0"
    }
  }
}

provider "scorecard" {
  api_token = "v1U37UDXfAHtABr7UJXaFdm5HDVqQPYFQ6Bo"
}

resource "scorecard_scorecard" "example" {
  name                = "Terraform Provider Scorecard"
  type                = "LEVEL"
  entity_filter_type  = "entity_types"
  entity_filter_type_identifiers = ["service"]
  evaluation_frequency_hours = 2
  empty_level_label   = "None"
  empty_level_color   = "#cccccc"
  levels = [{
    key   = "bronze"
    name  = "Bronze"
    color = "#cd7f32"
    rank  = 1
  }]
  checks = []
}
