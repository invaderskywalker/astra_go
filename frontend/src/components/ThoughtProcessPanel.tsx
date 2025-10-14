/* eslint-disable @typescript-eslint/no-explicit-any */
// ThoughtProcessPanel shows Astra's reasoning and intermediate steps.
import React from "react";
import RenderYamlView from "./RenderYamlView";

interface IntermediateMessage {
  text: string;
  timestamp: string;
}

interface ThoughtProcessPanelProps {
  thoughts: IntermediateMessage[];
}

const isJsonString = (str: string): boolean => {
  if (typeof str !== "string") return false;
  try {
    const parsed = JSON.parse(str);
    return typeof parsed === "object" && parsed !== null;
  } catch {
    return false;
  }
};

const parseMaybeJson = (input: any): any => {
  if (typeof input !== "string") return input;
  try {
    const parsed = JSON.parse(input);
    if (typeof parsed === "object" && parsed !== null) {
      for (const key in parsed) {
        if (typeof parsed[key] === "string" && isJsonString(parsed[key])) {
          parsed[key] = parseMaybeJson(parsed[key]);
        }
      }
    }
    return parsed;
  } catch {
    return input;
  }
};

const ThoughtProcessPanel: React.FC<ThoughtProcessPanelProps> = ({ thoughts }) => (
  <div className="thought-panel">
    <div className="thought-header">Astra's Thought Process</div>
    <div className="thought-messages">
      {thoughts.length === 0 ? (
        <div className="thought-empty">
          Astra's reasoning/steps will appear here as you chat
        </div>
      ) : (
        thoughts.map((m, i) => {
          const jsonMatch = m.text.match(/{[\s\S]*}$/);
          const maybeJson = jsonMatch ? jsonMatch[0] : m.text;
          const isJson = isJsonString(maybeJson);
          const parsedData = isJson ? parseMaybeJson(maybeJson) : maybeJson;
          return (
            <div key={i} className="thought-message">
              <div className="thought-time thought-time-top">{m.timestamp}</div>
              <span className="thought-text">
                {isJson && parsedData ? (
                  <RenderYamlView data={parsedData} />
                ) : (
                  m.text
                )}
              </span>
            </div>
          );
        })
      )}
    </div>
  </div>
);

export default ThoughtProcessPanel;
