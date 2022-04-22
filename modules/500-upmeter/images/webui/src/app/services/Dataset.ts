export class Dataset {
  data: any[] = []

  constructor() {}

  clear() {
    this.data = []
  }

  length(): number {
    return this.data.length
  }

  push(item: any) {
    this.data.push(item)
  }

  get(i: number): any {
    return this.data[i]
  }

  forEach(fn: (item: any, i?: number) => void) {
    this.data.forEach(fn)
  }
}
