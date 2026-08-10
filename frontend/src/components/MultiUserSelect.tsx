import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

interface Props {
  users: { id: number; display_name: string }[];
  selectedIds: number[];
  onChange: (ids: number[]) => void;
  placeholder?: string;
}

/** 多选负责人:已选用户标签(可点 x 移除)+ 下拉勾选列表(点击 toggle,去重) */
export default function MultiUserSelect({ users, selectedIds, onChange, placeholder }: Props) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  // 点击组件外关闭下拉
  useEffect(() => {
    const onClick = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onClick);
    return () => document.removeEventListener("mousedown", onClick);
  }, []);

  const selected = users.filter((u) => selectedIds.includes(u.id));
  const toggle = (id: number) => {
    if (selectedIds.includes(id)) onChange(selectedIds.filter((x) => x !== id));
    else onChange([...selectedIds, id]);
  };

  return (
    <div className="multi-user-select" ref={rootRef}>
      <div className="mus-tags-row">
        {/* 标签区只负责显示与删除(×);展开列表走独立"添加"按钮——点击标签不再触发任何操作 */}
        <div className="mus-tags">
          {selected.length === 0 && (
            <span className="mus-placeholder">{placeholder || t("taskDetail.assigneePlaceholder")}</span>
          )}
          {selected.map((u) => (
            <span key={u.id} className="mus-tag">
              {u.display_name}
              <button
                type="button"
                className="mus-tag-x"
                title={t("common.remove")}
                onClick={(e) => {
                  // 防御:仅当点击坐标落在 × 按钮内才移除。
                  // 实测某些环境对标签区一次点击会派发两个 click(第二个错位命中 ×),
                  // 坐标校验可拦截错位误删——真实用户点 × 的坐标必然在按钮内
                  const r = e.currentTarget.getBoundingClientRect();
                  if (e.clientX >= r.left - 4 && e.clientX <= r.right + 4 && e.clientY >= r.top - 4 && e.clientY <= r.bottom + 4) {
                    toggle(u.id);
                  }
                }}
              >
                ×
              </button>
            </span>
          ))}
        </div>
        <button
          type="button"
          className="mus-add-btn"
          onClick={() => setOpen((v) => !v)}
          title={t("common.add")}
        >
          ＋ {t("common.add")}
        </button>
      </div>
      {open && (
        <div className="mus-dropdown">
          {users.length === 0 ? (
            <div className="mus-empty">{t("common.noData")}</div>
          ) : (
            users.map((u) => (
              <label key={u.id} className="mus-option">
                <input
                  type="checkbox"
                  checked={selectedIds.includes(u.id)}
                  onChange={() => toggle(u.id)}
                />
                <span>{u.display_name}</span>
              </label>
            ))
          )}
        </div>
      )}
    </div>
  );
}
