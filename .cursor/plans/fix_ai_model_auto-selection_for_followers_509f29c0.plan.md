---
name: Fix AI Model Auto-Selection for Followers
overview: Fix the formData initialization in TraderConfigModal to properly auto-select the "Risk Management" model for followers using getDefaultRiskModel() instead of defaultModels[0], and update dependencies to ensure it recalculates when models are loaded.
todos:
  - id: "1"
    content: Fix TraderConfigViewModal - Delay onClose() to allow navigation
    status: completed
---

# Fix AI Model Auto-Selection for Followers

## Problem

The AI model is not being auto-selected for followers when they open the create trader modal. The validation message shows "Please fill in all required fields: AI Model" even though the "Risk Management" model should be pre-selected.

## Root Cause

In `TraderConfigModal.tsx`, the `useEffect` that initializes `formData` (line 264-303) uses `defaultModels[0]?.id || ''` for followers, which can be empty if:

1. `filteredModels` is empty when the effect runs (models not yet loaded)
2. The dependency array doesn't include `userIsFollower` or `filteredModels`, so it doesn't recalculate when they change
3. The separate auto-selection `useEffect` (line 113) might not trigger if `formData.ai_model` is already set to an empty string

## Solution

Update the formData initialization `useEffect` to:

1. Use `getDefaultRiskModel()` for followers instead of `defaultModels[0]?.id`
2. Add proper dependencies including `userIsFollower` and `filteredModels.length`
3. Ensure the model is selected even if `filteredModels` is initially empty (recalculate when models load)

## Implementation Steps

### 1. Update formData Initialization useEffect

**File**: `web/src/components/TraderConfigModal.tsx`

**Location**: Lines 264-303

**Changes**:

- Replace `defaultModels[0]?.id || '' `with `getDefaultRiskModel()` for followers
- Add `userIsFollower` and `filteredModels.length` to the dependency array
- Ensure the effect recalculates when models become available

**Code Change**:

````typescript
} else if (!isEditMode) {
  // For followers, use getDefaultRiskModel to prioritize "Risk Management" model
  const defaultAIModel = userIsFollower 
    ? getDefaultRiskModel() 
    : (availableModels[0]?.id || '')
  
  setFormData({
    trader_name: '',
    ai_model: defaultAIModel,
    exchange_id: availableExchanges[0]?.id || '',
    // ... rest of the fields
  })
}
}, [traderData, isEditMode, availableModels, availableExchanges, userIsFollower, filteredModels.length])
```

### 2. Consolidate Auto-Selection Logic

The separate `useEffect` at line 113-123 can be removed or simplified since the initialization `useEffect` will now handle it properly. However, keep it as a fallback to ensure the model is set even if the initialization happens before models are loaded.

**Alternative**: Keep both useEffects but ensure they work together:

- The initialization useEffect sets the default when formData is reset
- The auto-selection useEffect ensures it's set when models become available later

## Files to Modify

- `web/src/components/TraderConfigModal.tsx` - Update formData initialization to use `getDefaultRiskModel()` for followers and fix dependencies

## Testing Considerations

- Verify followers see "Risk Management" model auto-selected when opening create modal
- Verify the model is selected even if models load after modal opens
- Verify no validation error appears for AI Model field
- Verify the model selection persists when modal is reopened

````