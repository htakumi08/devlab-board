import { useEffect, useState } from "react";

import { fetchUserAgent } from "./api";

const initialMarker = "ExampleApp";

export function UserAgentLab() {
  const [marker, setMarker] = useState(initialMarker);
  const [serverUserAgent, setServerUserAgent] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [errorMessage, setErrorMessage] = useState("");

  const browserUserAgent = navigator.userAgent;
  const normalizedMarker = marker.trim();
  const browserMatched =
    normalizedMarker.length > 0 && browserUserAgent.includes(normalizedMarker);
  const serverMatched =
    normalizedMarker.length > 0 &&
    serverUserAgent !== null &&
    serverUserAgent.includes(normalizedMarker);

  useEffect(() => {
    void loadUserAgent();
  }, []);

  async function loadUserAgent() {
    setIsLoading(true);
    setErrorMessage("");
    setServerUserAgent(null);

    try {
      const payload = await fetchUserAgent();
      setServerUserAgent(payload.userAgent);
    } catch {
      setServerUserAgent(null);
      setErrorMessage("サーバーが受信した User-Agent を取得できませんでした。");
    } finally {
      setIsLoading(false);
    }
  }

  return (
    <section className="panel user-agent-lab" id="user-agent-lab">
      <div className="panel-header">
        <div>
          <p className="eyebrow">Request inspection</p>
          <h3>User-Agent Lab</h3>
        </div>
        <span>browser + Go</span>
      </div>

      <p className="lab-intro">
        ブラウザが参照する値と、Go API が HTTP リクエストで受信した値を比較します。
      </p>

      <label className="marker-field" htmlFor="user-agent-marker">
        <span>判定文字列</span>
        <input
          id="user-agent-marker"
          onChange={(event) => setMarker(event.target.value)}
          placeholder={initialMarker}
          type="text"
          value={marker}
        />
      </label>

      <div className="user-agent-grid">
        <article className="user-agent-result">
          <div className="result-heading">
            <h4>Browser</h4>
            <MatchBadge isMatched={browserMatched} isReady />
          </div>
          <code>{browserUserAgent}</code>
        </article>

        <article className="user-agent-result" aria-live="polite">
          <div className="result-heading">
            <h4>Server received</h4>
            <MatchBadge isMatched={serverMatched} isReady={serverUserAgent !== null} />
          </div>
          {isLoading && <p className="status-message">取得中です…</p>}
          {!isLoading && errorMessage && (
            <p className="status-message error-message">{errorMessage}</p>
          )}
          {!isLoading && serverUserAgent !== null && <code>{serverUserAgent}</code>}
        </article>
      </div>

      <button
        className="secondary-button reload-button"
        disabled={isLoading}
        onClick={() => void loadUserAgent()}
        type="button"
      >
        {isLoading ? "取得中" : "APIから再取得"}
      </button>

      <div className="lab-guide">
        <h4>確認手順</h4>
        <ol>
          <li>Chrome DevTools の Network conditions を開きます。</li>
          <li>User-Agent を上書きし、末尾に判定文字列を追加します。</li>
          <li>ページを再読み込みしてから、必要に応じてAPIを再取得します。</li>
        </ol>
      </div>

      <p className="lab-note" role="note">
        User-Agent の上書きはブラウザの名乗り方を変える確認です。Chrome が WebView
        そのものになるわけではありません。
      </p>
    </section>
  );
}

type MatchBadgeProps = {
  isMatched: boolean;
  isReady: boolean;
};

function MatchBadge({ isMatched, isReady }: MatchBadgeProps) {
  if (!isReady) {
    return <span className="match-badge pending">未取得</span>;
  }

  return (
    <span className={`match-badge ${isMatched ? "matched" : "unmatched"}`}>
      {isMatched ? "検出" : "未検出"}
    </span>
  );
}
