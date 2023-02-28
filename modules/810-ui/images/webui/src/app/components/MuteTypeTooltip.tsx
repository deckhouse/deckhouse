import React from "react"
import { i18n } from "../i18n"

export const MuteTypeTooltip = ({ muteTypeId }: { muteTypeId: string }) => {
  let items: any = i18n().mute.items
  let tooltip = {
    __html: items[muteTypeId].tooltip,
  }

  return <div dangerouslySetInnerHTML={tooltip}></div>
}
