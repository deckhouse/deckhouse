import dayjs from "dayjs";

export function formatTime(time: string, format: string = "DD.MM.YYYY HH:mm:ss"): string {
  return dayjs(time).format(format);
}

export function formatBytes(bytes: string | number, decimals = 0): string {
  if (typeof bytes == "string") {
    bytes = parseFloat(bytes);
  }

  if (bytes == 0) return "0 B";
  const k = 1024,
    sizes = ["B", "K", "M", "G", "T", "P"],
    i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(decimals)) + " " + sizes[i];
}

export function objectAsArray(object: any): { key: string; value: any }[] {
  return Object.keys(object).map((key) => {
    return { key, value: object[key] };
  });
}

export function arrayToObject(objectAsArray?: { key: string; value: any }[]): object | null {
  if (!objectAsArray) return null;

  return objectAsArray.reduce((obj: any, item): any => ((obj[item.key] = item.value), obj), {});
}
