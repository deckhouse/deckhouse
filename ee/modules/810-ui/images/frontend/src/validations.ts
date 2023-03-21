import { z } from "zod";
import { UpdateWindowsDays } from "@/consts";

const timeRegex = new RegExp(/^(?:\d|[01]\d|2[0-3]):[0-5]\d$/);
export const updateWindowSchema = z.object({
  days: z.enum(UpdateWindowsDays).array().nonempty(),
  from: z.string().regex(timeRegex, { message: "Некорректный формат" }),
  to: z.string().regex(timeRegex, { message: "Некорректный формат" }),
});
// TODO: validate to > from

export const nodeTemplateSchema = z.object({
  labelsAsArray: z
    .object({
      key: z.string().min(1),
      value: z.string().optional(),
    })
    .array(),
  annotationsAsArray: z
    .object({
      key: z.string().min(1),
      value: z.string().optional(),
    })
    .array(),
  taints: z
    .object({
      key: z.string().min(1),
      value: z.string().optional(),
      effect: z.string(),
    })
    .array()
    .optional(),
});
