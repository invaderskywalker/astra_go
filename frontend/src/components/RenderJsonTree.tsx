/* eslint-disable @typescript-eslint/no-explicit-any */
// RenderJsonTree.tsx
import React, { useState } from "react";

interface RenderJsonTreeProps {
  data: any;
  level?: number;
}

const isObject = (val: any) => val && typeof val === "object" && !Array.isArray(val);

const INDENT = 18;

const RenderJsonTree: React.FC<RenderJsonTreeProps> = ({ data, level = 0 }) => {
  const [collapsed, setCollapsed] = useState<{ [key: string]: boolean }>({});

  const renderCollapseToggle = (key: string, value: any) => {
    const isCollapsible = isObject(value) || Array.isArray(value);
    if (!isCollapsible) return null;
    const id = `${key}-${level}`;
    return (
      <span
        style={{ cursor: "pointer", marginRight: 6, color: "#9E9E9E" }}
        onClick={() =>
          setCollapsed(c => ({ ...c, [id]: !c[id] }))
        }
        title={collapsed[id] ? "Expand" : "Collapse"}
      >
        {collapsed[id] ? "â¶" : "â¼"}
      </span>
    );
  };

  if (Array.isArray(data)) {
    if (data.length === 0) return <span>[ ]</span>;
    return (
      <div style={{ marginLeft: level * INDENT }}>
        [
        {data.map((item, i) => (
          <div key={i} style={{ marginLeft: INDENT, borderLeft: "1.5px dashed #DDD", paddingLeft: 4 }}>
            <RenderJsonTree data={item} level={level + 1} />
            {i < data.length - 1 && ","}
          </div>
        ))}
        ]
      </div>
    );
  }

  if (isObject(data)) {
  const keys = Object.keys(data);
  if (keys.length === 0) return <span>{"{}"}</span>;
  return (
    <div style={{ marginLeft: level === 0 ? 0 : level * INDENT }}>
      {"{"}
      {keys.map((key, idx) => {
        const value = data[key];
        const isCollapsible = isObject(value) || Array.isArray(value);
        const id = `${key}-${level}`;
        const isCollapsed = collapsed[id];
        return (
          <div
            key={key}
            style={{
              display: "flex",
              alignItems: "flex-start",
              marginLeft: INDENT,
              borderLeft:
                isObject(value) || Array.isArray(value)
                  ? "1.5px dashed #EEE"
                  : undefined,
              paddingLeft: 4,
            }}
          >
            {renderCollapseToggle(key, value)}
            <span style={{ color: "#3C3C3C", fontWeight: 500 }}>{key}</span>
            <span>:&nbsp;</span>
            {isCollapsible ? (
              isCollapsed ? (
                <span style={{ color: "#888" }}>
                  {Array.isArray(value) ? "[..." : "{..."}
                </span>
              ) : (
                <RenderJsonTree data={value} level={level + 1} />
              )
            ) : (
              <span
                style={{
                  color:
                    typeof value === "string"
                      ? "#C2185B"
                      : "#1565C0",
                }}
              >
                {JSON.stringify(value)}
              </span>
            )}
            {idx < keys.length - 1 && ","}
          </div>
        );
      })}
      {"}"}
    </div>
  );
}


  // Primitives (number, string, boolean, null, undefined)
  return (
    <span style={{ color: typeof data === "string" ? "#C2185B" : typeof data === "number" ? "#1565C0" : "#388E3C" }}>
      {JSON.stringify(data)}
    </span>
  );
};

export default RenderJsonTree;
