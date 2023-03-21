import { setupWorker } from "msw";
import { rawHandlers } from "./handlers";

export const worker = setupWorker(...rawHandlers);
