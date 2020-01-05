import "./main.css";
import { Elm } from "./Main.elm";

Elm.Main.init({
  node: document.getElementById("root"),
  flags: process.env.ELM_APP_URL
});
