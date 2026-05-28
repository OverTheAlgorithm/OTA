import { useCallback, useReducer } from "react";
import type { Comment } from "@/lib/api";

// Reply cache: keep replies for one mount of CommentSection. Toggling a
// thread closed and reopening it should not refetch, but navigating away
// from the page (which unmounts the section) drops the cache so the next
// view starts fresh.

type ReplyState = {
  loaded: Comment[];
  nextCursor: string;
  isOpen: boolean;
  isLoading: boolean;
  error: string | null;
};

type Action =
  | { type: "open"; groupId: string }
  | { type: "close"; groupId: string }
  | { type: "loadStart"; groupId: string }
  | { type: "loadSuccess"; groupId: string; items: Comment[]; nextCursor: string }
  | { type: "loadError"; groupId: string; error: string }
  | { type: "appendReply"; groupId: string; reply: Comment }
  | { type: "updateReply"; groupId: string; reply: Comment }
  | { type: "removeReply"; groupId: string; replyId: string };

type State = Record<string, ReplyState>;

function emptyState(): ReplyState {
  return { loaded: [], nextCursor: "", isOpen: false, isLoading: false, error: null };
}

function reducer(state: State, action: Action): State {
  const current = state[action.groupId] ?? emptyState();
  switch (action.type) {
    case "open":
      return { ...state, [action.groupId]: { ...current, isOpen: true } };
    case "close":
      return { ...state, [action.groupId]: { ...current, isOpen: false } };
    case "loadStart":
      return { ...state, [action.groupId]: { ...current, isLoading: true, error: null } };
    case "loadSuccess":
      return {
        ...state,
        [action.groupId]: {
          ...current,
          isLoading: false,
          loaded: [...current.loaded, ...action.items],
          nextCursor: action.nextCursor,
          error: null,
        },
      };
    case "loadError":
      return { ...state, [action.groupId]: { ...current, isLoading: false, error: action.error } };
    case "appendReply":
      return {
        ...state,
        [action.groupId]: {
          ...current,
          loaded: [...current.loaded, action.reply],
        },
      };
    case "updateReply":
      return {
        ...state,
        [action.groupId]: {
          ...current,
          loaded: current.loaded.map((r) => (r.id === action.reply.id ? action.reply : r)),
        },
      };
    case "removeReply":
      return {
        ...state,
        [action.groupId]: {
          ...current,
          loaded: current.loaded.filter((r) => r.id !== action.replyId),
        },
      };
    default:
      return state;
  }
}

export interface CommentsCache {
  getReplies: (groupId: string) => ReplyState;
  setOpen: (groupId: string, open: boolean) => void;
  startLoad: (groupId: string) => void;
  finishLoad: (groupId: string, items: Comment[], nextCursor: string) => void;
  failLoad: (groupId: string, error: string) => void;
  appendReply: (groupId: string, reply: Comment) => void;
  updateReply: (groupId: string, reply: Comment) => void;
  removeReply: (groupId: string, replyId: string) => void;
}

export function useCommentsCache(): CommentsCache {
  const [state, dispatch] = useReducer(reducer, {});

  const getReplies = useCallback((groupId: string) => state[groupId] ?? emptyState(), [state]);
  const setOpen = useCallback((groupId: string, open: boolean) => {
    dispatch({ type: open ? "open" : "close", groupId });
  }, []);
  const startLoad = useCallback((groupId: string) => dispatch({ type: "loadStart", groupId }), []);
  const finishLoad = useCallback(
    (groupId: string, items: Comment[], nextCursor: string) =>
      dispatch({ type: "loadSuccess", groupId, items, nextCursor }),
    [],
  );
  const failLoad = useCallback(
    (groupId: string, error: string) => dispatch({ type: "loadError", groupId, error }),
    [],
  );
  const appendReply = useCallback(
    (groupId: string, reply: Comment) => dispatch({ type: "appendReply", groupId, reply }),
    [],
  );
  const updateReply = useCallback(
    (groupId: string, reply: Comment) => dispatch({ type: "updateReply", groupId, reply }),
    [],
  );
  const removeReply = useCallback(
    (groupId: string, replyId: string) => dispatch({ type: "removeReply", groupId, replyId }),
    [],
  );

  return {
    getReplies,
    setOpen,
    startLoad,
    finishLoad,
    failLoad,
    appendReply,
    updateReply,
    removeReply,
  };
}
