import React from "react"
import { i18n } from "../i18n"

export const GroupProbeTooltip = ({ groupName, probeName }: { groupName: string; probeName?: string }) => {
  let tooltip = { __html: "" }
  if (probeName === "__total__") {
    let el: any = i18n().group
    tooltip.__html = el[groupName]
  } else {
    let el: any = i18n().probe
    tooltip.__html = el[groupName][probeName]
  }

  return <div dangerouslySetInnerHTML={tooltip}></div>
}
