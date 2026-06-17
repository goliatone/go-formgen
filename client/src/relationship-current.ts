import type {
  CurrentOption,
  RelationshipCurrent,
  RelationshipCurrentItem,
} from "./config";

export function relationshipCurrentItemValue(item: RelationshipCurrentItem): string {
  return typeof item === "string" ? item.trim() : String(item.value ?? "").trim();
}

export function relationshipCurrentItemHasLabel(item: RelationshipCurrentItem): boolean {
  return typeof item !== "string" && String(item.label ?? "").trim() !== "";
}

export function relationshipCurrentItemToOption(
  item: RelationshipCurrentItem
): CurrentOption | null {
  if (typeof item === "string") {
    return item ? { value: item, label: item } : null;
  }
  const value = relationshipCurrentItemValue(item);
  const label = String(item.label ?? value).trim();
  return value ? { value, label: label || value } : null;
}

export function normalizeRelationshipCurrentOptions(
  current: RelationshipCurrent | undefined,
  allowMultiple: boolean
): CurrentOption[] {
  if (current == null) {
    return [];
  }
  const items = Array.isArray(current) ? current : [current];
  const options = items
    .map(relationshipCurrentItemToOption)
    .filter((option): option is CurrentOption => option != null);
  return allowMultiple ? options : options.slice(0, 1);
}

export function relationshipCurrentValues(
  current: RelationshipCurrent | undefined,
  allowMultiple = true
): string[] {
  if (current == null) {
    return [];
  }
  const items = Array.isArray(current) ? current : [current];
  const values = items
    .map(relationshipCurrentItemValue)
    .filter((value) => value.length > 0);
  return allowMultiple ? values : values.slice(0, 1);
}

export function relationshipCurrentValuesNeedingResolution(
  current: RelationshipCurrent | undefined
): string[] {
  if (current == null) {
    return [];
  }
  const items = Array.isArray(current) ? current : [current];
  return items
    .filter((item) => !relationshipCurrentItemHasLabel(item))
    .map(relationshipCurrentItemValue)
    .filter((value) => value.length > 0);
}
