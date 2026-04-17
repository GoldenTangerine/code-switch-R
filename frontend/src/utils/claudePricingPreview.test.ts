import { describe, expect, it } from 'vitest'
import {
  resolveClaudePricingPreviewTargetMode,
  summarizeClaudePricingPreviewTargets,
} from './claudePricingPreview'

describe('claudePricingPreview', () => {
  it('treats recognized rows as mapped targets', () => {
    expect(resolveClaudePricingPreviewTargetMode({
      display_name: 'Claude Opus 4.6',
      target_models: ['claude-opus-4-6'],
      input_cost_per_token: 0,
      output_cost_per_token: 0,
      cache_creation_input_token_cost: 0,
      cache_read_input_token_cost: 0,
      ephemeral_1h_cost_per_token: 0,
      is_recognized: true,
    })).toBe('mapped')
  })

  it('treats unmapped rows with fallback targets as custom additions', () => {
    expect(resolveClaudePricingPreviewTargetMode({
      display_name: 'Claude Unknown 9.9',
      target_models: ['claude-unknown-9-9'],
      input_cost_per_token: 0,
      output_cost_per_token: 0,
      cache_creation_input_token_cost: 0,
      cache_read_input_token_cost: 0,
      ephemeral_1h_cost_per_token: 0,
      is_recognized: false,
    })).toBe('custom')
  })

  it('treats unmapped rows without targets as missing', () => {
    expect(resolveClaudePricingPreviewTargetMode({
      display_name: '!!!',
      target_models: [],
      input_cost_per_token: 0,
      output_cost_per_token: 0,
      cache_creation_input_token_cost: 0,
      cache_read_input_token_cost: 0,
      ephemeral_1h_cost_per_token: 0,
      is_recognized: false,
    })).toBe('missing')
  })

  it('summarizes preview targets compactly', () => {
    expect(summarizeClaudePricingPreviewTargets({
      display_name: 'Claude Unknown 9.9',
      target_models: ['claude-unknown-9-9', 'anthropic/claude-unknown-9-9', 'vendor/claude-unknown-9-9'],
      input_cost_per_token: 0,
      output_cost_per_token: 0,
      cache_creation_input_token_cost: 0,
      cache_read_input_token_cost: 0,
      ephemeral_1h_cost_per_token: 0,
      is_recognized: false,
    })).toBe('claude-unknown-9-9, anthropic/claude-unknown-9-9 +1')
  })
})
