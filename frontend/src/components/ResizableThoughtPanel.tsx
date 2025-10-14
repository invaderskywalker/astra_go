/* eslint-disable @typescript-eslint/no-unused-vars */
// ResizableThoughtPanel provides a horizontally resizable panel wrapper.
import React, { useRef, useState, useEffect } from "react";

interface ResizableThoughtPanelProps {
  children: React.ReactNode;
}

const ResizableThoughtPanel: React.FC<ResizableThoughtPanelProps> = ({ children }) => {
  const panelRef = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState<number | null>(null);
  const [dragging, setDragging] = useState(false);
  const minWidth = 270;
  const maxWidth = 600;

  useEffect(() => {
    const savedWidth = window.localStorage.getItem('thoughtPanelWidth');
    if (savedWidth) {
      setWidth(Number(savedWidth));
    }
  }, []);

  useEffect(() => {
    if (width) {
      window.localStorage.setItem('thoughtPanelWidth', width.toString());
    }
  }, [width]);

  const startDrag = (e: React.MouseEvent) => {
    setDragging(true);
    document.body.style.cursor = 'col-resize';
  };

  useEffect(() => {
    const onMouseMove = (e: MouseEvent) => {
      if (!dragging || !panelRef.current) return;
      const parent = panelRef.current.parentElement;
      if (!parent) return;
      const parentRect = parent.getBoundingClientRect();
      const newWidth = parentRect.right - e.clientX;
      const clamped = Math.max(minWidth, Math.min(maxWidth, newWidth));
      setWidth(clamped);
    };
    const onMouseUp = () => {
      setDragging(false);
      document.body.style.cursor = '';
    };
    if (dragging) {
      window.addEventListener('mousemove', onMouseMove);
      window.addEventListener('mouseup', onMouseUp);
    }
    return () => {
      window.removeEventListener('mousemove', onMouseMove);
      window.removeEventListener('mouseup', onMouseUp);
    };
  }, [dragging]);

  return (
    <div
      className="thought-panel-resizable"
      ref={panelRef}
      style={width ? { width: width } : {}}
    >
      <div
        className="thought-panel-dragbar"
        onMouseDown={startDrag}
        role="separator"
        aria-orientation="vertical"
        tabIndex={0}
        title="Drag to resize"
      />
      {children}
    </div>
  );
};

export default ResizableThoughtPanel;
