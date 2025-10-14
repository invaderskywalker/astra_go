/* eslint-disable @typescript-eslint/no-unused-vars */
/* eslint-disable @typescript-eslint/no-explicit-any */
import React from "react";
import YAML from "js-yaml";

interface RenderYamlViewProps {
  data: any;
}

const RenderYamlView: React.FC<RenderYamlViewProps> = ({ data }) => {
  let yamlStr = "";
  try {
    yamlStr = YAML.dump(data, { indent: 2 });
  } catch (e) {
    yamlStr = "Error converting to YAML.";
  }

  return (
    <div
      style={{
        background: "#f8f9fb",
        borderRadius: "8px",
        padding: "12px",
        fontFamily: "monospace",
        fontSize: "13px",
        color: "#2c3e50",
        overflowX: "auto",
      }}
    >
      <pre>{yamlStr}</pre>
    </div>
  );
};

export default RenderYamlView;
