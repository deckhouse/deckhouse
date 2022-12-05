export function isNumber(x: any): x is number {
  return typeof x === "number"
}

export function isString(x: any): x is string {
  return typeof x === "string"
}
