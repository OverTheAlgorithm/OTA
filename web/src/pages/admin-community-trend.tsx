import { useEffect, useState, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";
import { LoadingState } from "@/components/spinner";
import * as ct from "@/lib/community-trend-api";

type Tab = "communities" | "worksheets" | "memes" | "trends" | "robots" | "tags" | "collect";

function todayISO(): string {
  return new Date().toISOString().slice(0, 10);
}

export function AdminCommunityTrendPage() {
  const { user, loading: authLoading } = useAuth();
  const navigate = useNavigate();
  const [tab, setTab] = useState<Tab>("worksheets");

  useEffect(() => {
    if (authLoading) return;
    if (!user || user.role !== "admin") navigate("/", { replace: true });
  }, [user, authLoading, navigate]);

  if (authLoading || !user || user.role !== "admin") {
    return (
      <div className="min-h-screen flex items-center justify-center bg-white">
        <LoadingState label="로딩 중" className="text-[#6b8db5]" />
      </div>
    );
  }

  const tabs: { key: Tab; label: string }[] = [
    { key: "worksheets", label: "워크시트" },
    { key: "communities", label: "커뮤니티" },
    { key: "tags", label: "태그/축" },
    { key: "memes", label: "밈" },
    { key: "trends", label: "트렌드" },
    { key: "robots", label: "Robots" },
    { key: "collect", label: "수동 실행" },
  ];

  return (
    <div className="min-h-screen p-8" style={{ backgroundColor: "var(--color-bg)", color: "var(--color-fg)" }}>
      <div className="max-w-4xl mx-auto space-y-6">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-bold">커뮤니티 트렌드</h1>
          <button onClick={() => navigate("/admin")} className="text-sm text-[#6b8db5] hover:text-[#1e3a5f]">
            ← 관리자 페이지
          </button>
        </div>

        <div className="flex gap-2 border-b border-gray-200">
          {tabs.map((t) => (
            <button
              key={t.key}
              onClick={() => setTab(t.key)}
              className={`px-4 py-2 text-sm font-medium ${
                tab === t.key ? "border-b-2 border-[#1e3a5f] text-[#1e3a5f]" : "text-gray-500"
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>

        {tab === "worksheets" && <WorksheetsTab />}
        {tab === "communities" && <CommunitiesTab />}
        {tab === "tags" && <TagsTab />}
        {tab === "memes" && <MemesTab />}
        {tab === "trends" && <TrendsTab />}
        {tab === "robots" && <RobotsTab />}
        {tab === "collect" && <CollectTab />}
      </div>
    </div>
  );
}

function ErrorBar({ msg }: { msg: string | null }) {
  if (!msg) return null;
  return <div className="rounded bg-red-50 text-red-700 text-sm px-3 py-2">{msg}</div>;
}

// ── Worksheets ────────────────────────────────────────────────────────────────

function WorksheetsTab() {
  const [date, setDate] = useState(todayISO());
  const [sheets, setSheets] = useState<ct.CTWorksheet[]>([]);
  const [communities, setCommunities] = useState<ct.CTCommunity[]>([]);
  const [tags, setTags] = useState<ct.CTTag[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [openId, setOpenId] = useState<number | null>(null);

  const load = useCallback(() => {
    setLoading(true);
    setError(null);
    Promise.all([ct.listWorksheets(date), ct.listCommunities(), ct.listTags()])
      .then(([s, c, t]) => {
        setSheets(s);
        setCommunities(c);
        setTags(t);
      })
      .catch((e) => setError(e instanceof Error ? e.message : "불러오기 실패"))
      .finally(() => setLoading(false));
  }, [date]);

  useEffect(() => {
    load();
  }, [load]);

  const commName = (id: number) => communities.find((c) => c.id === id)?.name ?? `#${id}`;

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <input type="date" value={date} onChange={(e) => setDate(e.target.value)} className="border rounded px-2 py-1 text-sm" />
        <button onClick={load} className="text-sm text-[#6b8db5]">새로고침</button>
      </div>
      <ErrorBar msg={error} />
      {loading ? (
        <LoadingState label="로딩 중" />
      ) : sheets.length === 0 ? (
        <p className="text-sm text-gray-500">이 날짜의 워크시트가 없습니다. (스케줄러가 매일 03시 KST에 생성)</p>
      ) : (
        <div className="space-y-2">
          {sheets.map((s) => (
            <div key={s.id} className="border rounded p-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="font-medium">{commName(s.community_id)}</span>
                  <Badge text={s.mode} />
                  <Badge text={s.status} tone={s.status === "confirmed" ? "green" : s.status === "suggested" ? "blue" : "gray"} />
                </div>
                <button onClick={() => setOpenId(openId === s.id ? null : s.id)} className="text-sm text-[#6b8db5]">
                  {openId === s.id ? "닫기" : "태깅"}
                </button>
              </div>
              {openId === s.id && (
                <TaggingPanel community_id={s.community_id} date={date} mode={s.mode} tags={tags} onDone={load} />
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function TaggingPanel({
  community_id,
  date,
  mode,
  tags,
  onDone,
}: {
  community_id: number;
  date: string;
  mode: string;
  tags: ct.CTTag[];
  onDone: () => void;
}) {
  const [counts, setCounts] = useState<Record<number, number>>({});
  const [total, setTotal] = useState(0);
  const [suggestion, setSuggestion] = useState<ct.CTSuggestion | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    ct.getSuggestion(community_id, date)
      .then((s) => {
        if (!s) return;
        setSuggestion(s);
        setTotal(s.total_posts);
        const seed: Record<number, number> = {};
        (s.output.tags ?? []).forEach((t) => {
          if (t.tag_id > 0) seed[t.tag_id] = t.count;
        });
        setCounts(seed);
      })
      .catch(() => {});
  }, [community_id, date]);

  const setCount = (tagId: number, v: number) => setCounts((p) => ({ ...p, [tagId]: v }));

  const confirm = async () => {
    setSaving(true);
    setError(null);
    try {
      await ct.confirmWorksheet({
        community_id,
        stat_date: date,
        mode,
        source: mode === "auto" ? "hybrid" : "human",
        total_posts: total,
        counts: Object.entries(counts)
          .map(([tag_id, count]) => ({ tag_id: Number(tag_id), count }))
          .filter((c) => c.count > 0),
      });
      onDone();
    } catch (e) {
      setError(e instanceof Error ? e.message : "확정 실패");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="mt-3 border-t pt-3 space-y-3">
      {suggestion && (
        <p className="text-xs text-gray-500">
          AI 제안 {suggestion.output.tags?.length ?? 0}개 · 신규 항목 {suggestion.total_posts}건
          {(suggestion.output.meme_candidates?.length ?? 0) > 0 &&
            ` · 밈 후보 ${suggestion.output.meme_candidates!.length}개`}
        </p>
      )}
      <ErrorBar msg={error} />
      <div className="flex items-center gap-2 text-sm">
        <label>일일 총 글수</label>
        <input
          type="number"
          value={total}
          onChange={(e) => setTotal(Number(e.target.value))}
          className="border rounded px-2 py-1 w-24"
        />
      </div>
      <div className="grid grid-cols-2 gap-2 max-h-72 overflow-y-auto">
        {tags.map((t) => (
          <div key={t.id} className="flex items-center gap-2 text-sm">
            <span className="flex-1 truncate" title={t.name}>
              {t.name}
            </span>
            <input
              type="number"
              min={0}
              value={counts[t.id] ?? 0}
              onChange={(e) => setCount(t.id, Number(e.target.value))}
              className="border rounded px-2 py-1 w-16"
            />
          </div>
        ))}
      </div>
      <button
        onClick={confirm}
        disabled={saving}
        className="bg-[#1e3a5f] text-white text-sm rounded px-4 py-2 disabled:opacity-50"
      >
        {saving ? "확정 중…" : "확정·발행"}
      </button>
    </div>
  );
}

// ── Communities ───────────────────────────────────────────────────────────────

function CommunitiesTab() {
  const [communities, setCommunities] = useState<ct.CTCommunity[]>([]);
  const [tags, setTags] = useState<ct.CTTag[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState({ key: "", name: "", home_url: "" });

  const load = () => {
    Promise.all([ct.listCommunities(), ct.listTags()])
      .then(([c, t]) => {
        setCommunities(c);
        setTags(t);
      })
      .catch((e) => setError(e instanceof Error ? e.message : "불러오기 실패"));
  };
  useEffect(load, []);

  const create = async () => {
    setError(null);
    try {
      await ct.createCommunity(form.key, form.name, form.home_url);
      setForm({ key: "", name: "", home_url: "" });
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "생성 실패");
    }
  };

  const toggleMeta = async (c: ct.CTCommunity, tagId: number) => {
    const cur = new Set(c.meta_tag_ids ?? []);
    cur.has(tagId) ? cur.delete(tagId) : cur.add(tagId);
    try {
      await ct.setMetaTags(c.id, [...cur]);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "메타태그 변경 실패");
    }
  };

  return (
    <div className="space-y-4">
      <ErrorBar msg={error} />
      <div className="border rounded p-3 flex gap-2 items-end text-sm">
        <div>
          <label className="block text-xs text-gray-500">key</label>
          <input value={form.key} onChange={(e) => setForm({ ...form, key: e.target.value })} className="border rounded px-2 py-1" placeholder="fmkorea" />
        </div>
        <div>
          <label className="block text-xs text-gray-500">이름</label>
          <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} className="border rounded px-2 py-1" placeholder="에펨코리아" />
        </div>
        <div className="flex-1">
          <label className="block text-xs text-gray-500">홈 URL</label>
          <input value={form.home_url} onChange={(e) => setForm({ ...form, home_url: e.target.value })} className="border rounded px-2 py-1 w-full" />
        </div>
        <button onClick={create} className="bg-[#1e3a5f] text-white rounded px-3 py-1">추가</button>
      </div>

      {communities.map((c) => (
        <div key={c.id} className="border rounded p-3 space-y-2">
          <div className="flex items-center gap-2">
            <span className="font-medium">{c.name}</span>
            <span className="text-xs text-gray-400">{c.key}</span>
            {!c.enabled && <Badge text="비활성" tone="gray" />}
          </div>
          <div className="flex flex-wrap gap-1">
            {tags
              .filter((t) => t.axis_id <= 3 /* meta axes: leaning/political/age */)
              .map((t) => {
                const on = (c.meta_tag_ids ?? []).includes(t.id);
                return (
                  <button
                    key={t.id}
                    onClick={() => toggleMeta(c, t.id)}
                    className={`text-xs rounded-full px-2 py-0.5 border ${on ? "bg-[#1e3a5f] text-white border-[#1e3a5f]" : "text-gray-500 border-gray-300"}`}
                  >
                    {t.name}
                  </button>
                );
              })}
          </div>
        </div>
      ))}
    </div>
  );
}

// ── Memes ─────────────────────────────────────────────────────────────────────

function MemesTab() {
  const [candidates, setCandidates] = useState<ct.CTMemeCandidate[]>([]);
  const [memes, setMemes] = useState<ct.CTMeme[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [newMeme, setNewMeme] = useState("");

  const load = () => {
    Promise.all([ct.listMemeCandidates(), ct.listMemes(true)])
      .then(([c, m]) => {
        setCandidates(c);
        setMemes(m);
      })
      .catch((e) => setError(e instanceof Error ? e.message : "불러오기 실패"));
  };
  useEffect(load, []);

  const wrap = (fn: () => Promise<unknown>) => async () => {
    setError(null);
    try {
      await fn();
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "실패");
    }
  };

  return (
    <div className="space-y-5">
      <ErrorBar msg={error} />
      <section>
        <h2 className="font-semibold mb-2">밈 후보 (AI 발굴)</h2>
        {candidates.length === 0 ? (
          <p className="text-sm text-gray-500">후보 없음</p>
        ) : (
          <div className="space-y-1">
            {candidates.map((c) => (
              <div key={c.id} className="flex items-center gap-2 text-sm border rounded px-3 py-2">
                <span className="flex-1">
                  {c.expression} <span className="text-xs text-gray-400">({c.hit_count}회)</span>
                </span>
                <button onClick={wrap(() => ct.promoteMemeCandidate(c.id, c.expression, []))} className="text-[#1e3a5f]">
                  승격
                </button>
                <button onClick={wrap(() => ct.rejectMemeCandidate(c.id))} className="text-red-500">
                  거부
                </button>
              </div>
            ))}
          </div>
        )}
      </section>

      <section>
        <h2 className="font-semibold mb-2">확정 밈</h2>
        <div className="flex gap-2 mb-2 text-sm">
          <input value={newMeme} onChange={(e) => setNewMeme(e.target.value)} placeholder="밈 직접 추가" className="border rounded px-2 py-1" />
          <button
            onClick={wrap(async () => {
              if (newMeme.trim()) {
                await ct.createMeme(newMeme.trim(), []);
                setNewMeme("");
              }
            })}
            className="bg-[#1e3a5f] text-white rounded px-3 py-1"
          >
            추가
          </button>
        </div>
        <div className="space-y-1">
          {memes.map((m) => (
            <div key={m.id} className="flex items-center gap-2 text-sm border rounded px-3 py-2">
              <span className="flex-1">
                {m.name}
                {m.aliases.length > 0 && <span className="text-xs text-gray-400"> ({m.aliases.join(", ")})</span>}
              </span>
              {m.status === "retired" ? (
                <Badge text="은퇴" tone="gray" />
              ) : (
                <button onClick={wrap(() => ct.retireMeme(m.id))} className="text-red-500">
                  은퇴
                </button>
              )}
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}

// ── Trends ────────────────────────────────────────────────────────────────────

function TrendsTab() {
  const [communities, setCommunities] = useState<ct.CTCommunity[]>([]);
  const [communityId, setCommunityId] = useState<number | null>(null);
  const [trends, setTrends] = useState<ct.CTTagTrend[]>([]);
  const [error, setError] = useState<string | null>(null);
  const to = todayISO();
  const from = new Date(Date.now() - 7 * 86400000).toISOString().slice(0, 10);

  useEffect(() => {
    ct.listCommunities()
      .then((c) => {
        setCommunities(c);
        if (c.length > 0) setCommunityId(c[0].id);
      })
      .catch((e) => setError(e instanceof Error ? e.message : "불러오기 실패"));
  }, []);

  useEffect(() => {
    if (communityId == null) return;
    ct.communityTrends(communityId, from, to)
      .then(setTrends)
      .catch((e) => setError(e instanceof Error ? e.message : "불러오기 실패"));
  }, [communityId, from, to]);

  return (
    <div className="space-y-4">
      <ErrorBar msg={error} />
      <select
        value={communityId ?? ""}
        onChange={(e) => setCommunityId(Number(e.target.value))}
        className="border rounded px-2 py-1 text-sm"
      >
        {communities.map((c) => (
          <option key={c.id} value={c.id}>
            {c.name}
          </option>
        ))}
      </select>
      <p className="text-xs text-gray-400">{from} ~ {to} (최근 7일)</p>
      {trends.length === 0 ? (
        <p className="text-sm text-gray-500">데이터 없음 (임계값 미만 태그는 숨김)</p>
      ) : (
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-gray-500 border-b">
              <th className="py-1">태그</th>
              <th>최근</th>
              <th>전일 대비</th>
              <th>전주 대비</th>
            </tr>
          </thead>
          <tbody>
            {trends.map((t) => (
              <tr key={t.tag_id} className="border-b">
                <td className="py-1">{t.tag_name}</td>
                <td>{t.latest}</td>
                <td><Delta v={t.delta_prev_day} /></td>
                <td><Delta v={t.delta_prev_week} /></td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

function Delta({ v }: { v: number }) {
  const color = v > 0 ? "text-red-500" : v < 0 ? "text-blue-500" : "text-gray-400";
  return <span className={color}>{v > 0 ? `▲${v}` : v < 0 ? `▼${-v}` : "—"}</span>;
}

function Badge({ text, tone = "blue" }: { text: string; tone?: "blue" | "green" | "gray" }) {
  const tones = {
    blue: "bg-blue-50 text-blue-600",
    green: "bg-green-50 text-green-600",
    gray: "bg-gray-100 text-gray-500",
  };
  return <span className={`text-xs rounded-full px-2 py-0.5 ${tones[tone]}`}>{text}</span>;
}

function RobotsTab() {
  const [data, setData] = useState<ct.CTRobotsData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(() => {
    setLoading(true);
    setError(null);
    ct.listRobotsStatus()
      .then(setData)
      .catch((e) => setError(e instanceof Error ? e.message : "불러오기 실패"))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="font-semibold text-base text-[#1e3a5f]">현재 robots.txt 상태</h2>
        <button onClick={load} className="text-sm text-[#6b8db5] hover:underline">새로고침</button>
      </div>
      <ErrorBar msg={error} />

      {loading ? (
        <LoadingState label="로딩 중" />
      ) : !data ? (
        <p className="text-sm text-gray-500">데이터가 없습니다.</p>
      ) : (
        <div className="space-y-6">
          <div className="border rounded overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-gray-500 border-b bg-gray-50">
                  <th className="p-3">커뮤니티</th>
                  <th className="p-3">수집 허용 여부</th>
                  <th className="p-3">확인 시각</th>
                  <th className="p-3">메모</th>
                  <th className="p-3">해시값</th>
                </tr>
              </thead>
              <tbody>
                {data.status.map((s) => (
                  <tr key={s.community_id} className="border-b hover:bg-gray-50/50">
                    <td className="p-3 font-medium text-[#1e3a5f]">{s.community_name}</td>
                    <td className="p-3">
                      <Badge
                        text={s.allowed ? "허용" : "차단됨 (수동 모드)"}
                        tone={s.allowed ? "green" : "gray"}
                      />
                    </td>
                    <td className="p-3 text-xs text-gray-500">
                      {new Date(s.checked_at).toLocaleString()}
                    </td>
                    <td className="p-3 text-gray-700">{s.note || "—"}</td>
                    <td className="p-3 text-xs font-mono text-gray-400 max-w-[120px] truncate" title={s.snapshot_hash}>
                      {s.snapshot_hash ? s.snapshot_hash.slice(0, 10) : "—"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="space-y-3">
            <h2 className="font-semibold text-base text-[#1e3a5f]">최근 상태 전이 이력</h2>
            <div className="border rounded overflow-hidden max-h-96 overflow-y-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left text-gray-500 border-b bg-gray-50">
                    <th className="p-3">커뮤니티</th>
                    <th className="p-3">상태 변경</th>
                    <th className="p-3">변경 시각</th>
                  </tr>
                </thead>
                <tbody>
                  {data.transitions.length === 0 ? (
                    <tr>
                      <td colSpan={3} className="p-3 text-center text-gray-500">기록된 상태 변경이 없습니다.</td>
                    </tr>
                  ) : (
                    data.transitions.map((t) => (
                      <tr key={t.id} className="border-b hover:bg-gray-50/50">
                        <td className="p-3 font-medium text-[#1e3a5f]">{t.community_name}</td>
                        <td className="p-3">
                          <span className="flex items-center gap-2">
                            <Badge
                              text={t.from_allowed === null ? "최초" : t.from_allowed ? "허용" : "차단"}
                              tone={t.from_allowed === null ? "blue" : t.from_allowed ? "green" : "gray"}
                            />
                            <span className="text-gray-400">→</span>
                            <Badge
                              text={t.to_allowed ? "허용" : "차단"}
                              tone={t.to_allowed ? "green" : "gray"}
                            />
                          </span>
                        </td>
                        <td className="p-3 text-xs text-gray-500">
                          {new Date(t.changed_at).toLocaleString()}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ── Tags & Axes ───────────────────────────────────────────────────────────────

function TagsTab() {
  const [axes, setAxes] = useState<ct.CTAxis[]>([]);
  const [tags, setTags] = useState<ct.CTTag[]>([]);
  const [selectedAxisId, setSelectedAxisId] = useState<number | "all">("all");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  // Forms
  const [axisForm, setAxisForm] = useState({ key: "", label: "", displayOrder: 0, type: "topic" });
  const [tagForm, setTagForm] = useState({ axisId: "", name: "", description: "" });

  // Inline editing state
  const [editingTagId, setEditingTagId] = useState<number | null>(null);
  const [editForm, setEditForm] = useState({ name: "", description: "" });

  const load = useCallback(() => {
    setLoading(true);
    setError(null);
    Promise.all([ct.listAxes(), ct.listTags()])
      .then(([a, t]) => {
        setAxes(a);
        setTags(t);
      })
      .catch((e) => setError(e instanceof Error ? e.message : "불러오기 실패"))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const createAxis = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!axisForm.key.trim() || !axisForm.label.trim()) {
      setError("축 키와 레이블을 입력하세요.");
      return;
    }
    setError(null);
    try {
      await ct.createAxis(axisForm.key.trim(), axisForm.label.trim(), axisForm.displayOrder, axisForm.type);
      setAxisForm({ key: "", label: "", displayOrder: 0, type: "topic" });
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "축 생성 실패");
    }
  };

  const createTag = async (e: React.FormEvent) => {
    e.preventDefault();
    const axisId = Number(tagForm.axisId);
    if (!axisId || !tagForm.name.trim()) {
      setError("축을 선택하고 태그 이름을 입력하세요.");
      return;
    }
    setError(null);
    try {
      await ct.createTag(axisId, tagForm.name.trim(), tagForm.description.trim());
      setTagForm((prev) => ({ ...prev, name: "", description: "" }));
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "태그 생성 실패");
    }
  };

  const updateTag = async (id: number) => {
    if (!editForm.name.trim()) {
      setError("태그 이름을 입력하세요.");
      return;
    }
    setError(null);
    try {
      await ct.updateTag(id, editForm.name.trim(), editForm.description.trim());
      setEditingTagId(null);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "태그 수정 실패");
    }
  };

  const deleteTag = async (id: number) => {
    if (!confirm("정말 이 태그를 삭제하시겠습니까?")) return;
    setError(null);
    try {
      await ct.deleteTag(id);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "태그 삭제 실패");
    }
  };

  const filteredTags = selectedAxisId === "all"
    ? tags
    : tags.filter((t) => t.axis_id === selectedAxisId);

  const getAxisLabel = (axisId: number) => {
    return axes.find((a) => a.id === axisId)?.label ?? `축 #${axisId}`;
  };

  const startEdit = (tag: ct.CTTag) => {
    setEditingTagId(tag.id);
    setEditForm({ name: tag.name, description: tag.description });
  };

  return (
    <div className="space-y-6">
      <ErrorBar msg={error} />

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* Axis Creation Form */}
        <form onSubmit={createAxis} className="border rounded p-4 space-y-3 text-sm">
          <h3 className="font-semibold text-base text-[#1e3a5f]">분류 축 추가</h3>
          <div>
            <label className="block text-xs text-gray-500 mb-1">축 고유 키</label>
            <input
              value={axisForm.key}
              onChange={(e) => setAxisForm({ ...axisForm, key: e.target.value })}
              className="border rounded px-2 py-1 w-full bg-white text-black"
              placeholder="e.g. leaning, gender, political"
            />
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">축 레이블</label>
            <input
              value={axisForm.label}
              onChange={(e) => setAxisForm({ ...axisForm, label: e.target.value })}
              className="border rounded px-2 py-1 w-full bg-white text-black"
              placeholder="e.g. 성향, 성별, 정치 성향"
            />
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">정렬 순서</label>
            <input
              type="number"
              value={axisForm.displayOrder}
              onChange={(e) => setAxisForm({ ...axisForm, displayOrder: Number(e.target.value) })}
              className="border rounded px-2 py-1 w-full bg-white text-black"
            />
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">축 분류 용도</label>
            <select
              value={axisForm.type}
              onChange={(e) => setAxisForm({ ...axisForm, type: e.target.value })}
              className="border rounded px-2 py-1 w-full bg-white text-black"
            >
              <option value="topic">일반 게시글 논제 (topic)</option>
              <option value="meta">커뮤니티 메타 성향 (meta)</option>
            </select>
          </div>
          <button type="submit" className="bg-[#1e3a5f] text-white rounded px-4 py-2 w-full font-medium hover:bg-[#152a45] transition-colors">
            축 추가
          </button>
        </form>

        {/* Tag Creation Form */}
        <form onSubmit={createTag} className="border rounded p-4 space-y-3 text-sm">
          <h3 className="font-semibold text-base text-[#1e3a5f]">공통 태그 추가</h3>
          <div>
            <label className="block text-xs text-gray-500 mb-1">소속 축</label>
            <select
              value={tagForm.axisId}
              onChange={(e) => setTagForm({ ...tagForm, axisId: e.target.value })}
              className="border rounded px-2 py-1 w-full bg-white text-black"
            >
              <option value="">축을 선택하세요</option>
              {axes.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.label} ({a.key})
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">태그 이름</label>
            <input
              value={tagForm.name}
              onChange={(e) => setTagForm({ ...tagForm, name: e.target.value })}
              className="border rounded px-2 py-1 w-full bg-white text-black"
              placeholder="e.g. 남성향, 여성향, 진보, 보수"
            />
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">설명</label>
            <input
              value={tagForm.description}
              onChange={(e) => setTagForm({ ...tagForm, description: e.target.value })}
              className="border rounded px-2 py-1 w-full bg-white text-black"
              placeholder="태그에 대한 간략한 설명"
            />
          </div>
          <button type="submit" className="bg-[#1e3a5f] text-white rounded px-4 py-2 w-full font-medium hover:bg-[#152a45] transition-colors">
            태그 추가
          </button>
        </form>
      </div>

      {/* Tags List */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="font-semibold text-base text-[#1e3a5f]">등록된 태그 목록</h3>
          <div className="flex items-center gap-2 text-sm">
            <span className="text-gray-500">축 필터:</span>
            <select
              value={selectedAxisId}
              onChange={(e) => setSelectedAxisId(e.target.value === "all" ? "all" : Number(e.target.value))}
              className="border rounded px-2 py-1 bg-white text-black"
            >
              <option value="all">전체보기</option>
              {axes.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.label}
                </option>
              ))}
            </select>
          </div>
        </div>

        {loading ? (
          <LoadingState label="로딩 중" />
        ) : filteredTags.length === 0 ? (
          <p className="text-sm text-gray-500 border rounded p-8 text-center bg-gray-50/50">등록된 태그가 없습니다.</p>
        ) : (
          <div className="border rounded overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-gray-500 border-b bg-gray-50">
                  <th className="p-3">소속 축</th>
                  <th className="p-3">태그 이름</th>
                  <th className="p-3">설명</th>
                  <th className="p-3">생성자</th>
                  <th className="p-3 text-right">작업</th>
                </tr>
              </thead>
              <tbody>
                {filteredTags.map((t) => {
                  const isEditing = editingTagId === t.id;
                  return (
                    <tr key={t.id} className="border-b hover:bg-gray-50/30">
                      <td className="p-3 font-medium text-[#1e3a5f]">{getAxisLabel(t.axis_id)}</td>
                      <td className="p-3">
                        {isEditing ? (
                          <input
                            value={editForm.name}
                            onChange={(e) => setEditForm({ ...editForm, name: e.target.value })}
                            className="border rounded px-2 py-0.5 bg-white text-black w-full"
                          />
                        ) : (
                          <span className="font-semibold text-gray-800">{t.name}</span>
                        )}
                      </td>
                      <td className="p-3">
                        {isEditing ? (
                          <input
                            value={editForm.description}
                            onChange={(e) => setEditForm({ ...editForm, description: e.target.value })}
                            className="border rounded px-2 py-0.5 bg-white text-black w-full"
                          />
                        ) : (
                          <span className="text-gray-600">{t.description || "—"}</span>
                        )}
                      </td>
                      <td className="p-3 text-xs text-gray-400">{t.created_by}</td>
                      <td className="p-3 text-right space-x-2">
                        {isEditing ? (
                          <>
                            <button
                              onClick={() => updateTag(t.id)}
                              className="text-green-600 hover:text-green-800 font-semibold"
                            >
                              저장
                            </button>
                            <button
                              onClick={() => setEditingTagId(null)}
                              className="text-gray-500 hover:text-gray-700 font-semibold"
                            >
                              취소
                            </button>
                          </>
                        ) : (
                          <>
                            <button
                              onClick={() => startEdit(t)}
                              className="text-[#6b8db5] hover:text-[#1e3a5f] font-semibold"
                            >
                              수정
                            </button>
                            <button
                              onClick={() => deleteTag(t.id)}
                              className="text-red-500 hover:text-red-700 font-semibold"
                            >
                              삭제
                            </button>
                          </>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}

function CollectTab() {
  const [date, setDate] = useState<string>(todayISO());
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [results, setResults] = useState<ct.CTCommunityResult[] | null>(null);

  const runCollection = () => {
    setLoading(true);
    setError(null);
    setResults(null);
    ct.triggerCollect(date)
      .then((res) => {
        setResults(res);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "수집 실행 중 오류가 발생했습니다.");
      })
      .finally(() => {
        setLoading(false);
      });
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "suggested":
        return <span className="px-2 py-1 text-xs font-semibold rounded-full bg-green-100 text-green-800">Suggested (자동 제안)</span>;
      case "pending":
        return <span className="px-2 py-1 text-xs font-semibold rounded-full bg-yellow-100 text-yellow-800">Pending (수동 대기)</span>;
      case "error":
        return <span className="px-2 py-1 text-xs font-semibold rounded-full bg-red-100 text-red-800">Error (에러)</span>;
      default:
        return <span className="px-2 py-1 text-xs font-semibold rounded-full bg-gray-100 text-gray-800">{status}</span>;
    }
  };

  return (
    <div className="space-y-6">
      <div className="bg-blue-50 border border-blue-200 text-blue-800 p-4 rounded text-sm space-y-1">
        <p className="font-semibold">💡 수동 수집 프로세스 안내</p>
        <p>수집 실행 시 어댑터 크롤링과 AI 분석(Gemini)이 진행되며 약 15~30초 가량 소요됩니다.</p>
        <p className="font-medium text-blue-900 mt-1">※ 수동 실행이 완료되더라도 임시 제안 상태일 뿐이며, <strong>'워크시트'</strong> 탭에서 최종적으로 <strong>[승인(Confirm)]</strong> 버튼을 클릭해야 최종 통계 데이터가 DB에 반영됩니다.</p>
      </div>

      <div className="flex items-center gap-4 bg-white p-4 border rounded shadow-sm">
        <div className="flex flex-col gap-1">
          <label className="text-xs font-semibold text-gray-500">수집 대상 날짜</label>
          <input
            type="date"
            value={date}
            onChange={(e) => setDate(e.target.value)}
            className="border rounded px-3 py-1.5 text-sm outline-none focus:border-[#1e3a5f]"
          />
        </div>
        <button
          onClick={runCollection}
          disabled={loading}
          className="self-end px-5 py-2 text-sm font-semibold rounded text-white bg-[#1e3a5f] hover:bg-[#152943] disabled:bg-gray-300 transition-colors"
        >
          {loading ? "수집 분석 진행 중..." : "수집 분석 실행"}
        </button>
      </div>

      <ErrorBar msg={error} />

      {loading && (
        <div className="py-12 flex justify-center">
          <LoadingState label="커뮤니티 트렌드 수집 및 AI 태깅 분석을 수행 중입니다. 잠시만 기다려 주세요..." />
        </div>
      )}

      {results && (
        <div className="space-y-4">
          <h3 className="font-semibold text-base text-[#1e3a5f]">수집 실행 결과</h3>
          <div className="border rounded overflow-hidden bg-white">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 border-b">
                <tr>
                  <th className="px-4 py-3 text-left font-medium text-gray-600">커뮤니티 키</th>
                  <th className="px-4 py-3 text-left font-medium text-gray-600">작동 모드</th>
                  <th className="px-4 py-3 text-left font-medium text-gray-600">결과 상태</th>
                  <th className="px-4 py-3 text-left font-medium text-gray-600">상세 이유 / 결과</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {results.map((r) => (
                  <tr key={r.key} className="hover:bg-gray-50">
                    <td className="px-4 py-3 font-semibold text-[#1e3a5f]">{r.key}</td>
                    <td className="px-4 py-3 text-gray-600">{r.mode === "auto" ? "자동 (AI)" : "수동"}</td>
                    <td className="px-4 py-3">{getStatusBadge(r.status)}</td>
                    <td className="px-4 py-3 text-gray-500">{r.reason}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
