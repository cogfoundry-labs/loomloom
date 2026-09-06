/**
 * All configuration options: https://remotion.dev/docs/config
 * (Node.JS APIs ignore this file — pass options directly there.)
 */

import { Config } from "@remotion/cli/config";

Config.setRspack(true);
Config.setVideoImageFormat("jpeg");
Config.setOverwriteOutput(true);

// Use the case study's real assets — no duplicate copy in this project.
Config.setPublicDir("../assets");
