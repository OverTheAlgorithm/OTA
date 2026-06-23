import { useEffect, useState, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/auth-context";
import { LoadingState } from "@/components/spinner";
import * as ct from "@/lib/community-trend-api";

type Tab = "communities" | "worksheets" | "memes" | "trends";

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
    { key: "memes", label: "밈" },
    { key: "trends", label: "트렌드" },
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
        {tab === "memes" && <MemesTab />}
        {tab === "trends" && <TrendsTab />}
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
