import type { ReactNode } from "react";
import { Checkbox, TreeNode } from "@carbon/react";

export type OrgUnitNodeData = {
  id: string;
  name: string;
  children?: OrgUnitNodeData[];
};

interface OrgUnitNodeProps {
  node: OrgUnitNodeData;
  searchTerm?: string;
  selectedOrgUnit?: string;
  expandedNodes?: Set<string>;
  onSelect: (name: string) => void;
  onToggleNode?: (nodeId: string, isExpanded: boolean) => void;
  renderRecursive: (nodes: OrgUnitNodeData[], idPrefix?: string) => ReactNode[];
  idPrefix?: string;
}

export const OrgUnitNode = ({
  node,
  searchTerm,
  selectedOrgUnit,
  expandedNodes,
  onSelect,
  onToggleNode,
  renderRecursive,
  idPrefix = "check",
}: OrgUnitNodeProps) => {
  const isNodeExpanded =
    Boolean(searchTerm) || (expandedNodes ? expandedNodes.has(node.id) : undefined);

  return (
    <TreeNode
      key={node.id}
      id={node.id}
      isExpanded={isNodeExpanded}
      onToggle={(_event: unknown, data?: { id?: string; isExpanded?: boolean }) => {
        if (onToggleNode && node.id) {
          onToggleNode(node.id, Boolean(data?.isExpanded));
        }
      }}
      label={
        <div
          className="org-tree-node-label"
          onClick={(e) => e.stopPropagation()}
        >
          <Checkbox
            id={`${idPrefix}-${node.id}`}
            labelText={node.name}
            checked={selectedOrgUnit === node.name}
            onChange={() => onSelect(node.name)}
          />
        </div>
      }
    >
      {node.children && node.children.length > 0
        ? renderRecursive(node.children, idPrefix)
        : null}
    </TreeNode>
  );
};
