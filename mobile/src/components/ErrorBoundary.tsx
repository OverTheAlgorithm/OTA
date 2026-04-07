// Extracted from: web/src/components/error-boundary.tsx
// Differences: React Native View/Text/Pressable instead of HTML elements

import { Component, type ErrorInfo, type ReactNode } from "react";
import { View, Text, Pressable, StyleSheet } from "react-native";

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("[ErrorBoundary] Caught rendering error:", error, info.componentStack);
  }

  handleRetry = () => {
    this.setState({ hasError: false, error: null });
  };

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback;
      }
      return (
        <ErrorFallback
          error={this.state.error}
          onRetry={this.handleRetry}
        />
      );
    }
    return this.props.children;
  }
}

interface ErrorFallbackProps {
  error: Error | null;
  onRetry?: () => void;
}

export function ErrorFallback({ onRetry }: ErrorFallbackProps) {
  return (
    <View style={styles.container}>
      <Text style={styles.icon}>!</Text>
      <Text style={styles.title}>문제가 발생했어요</Text>
      <Text style={styles.subtitle}>
        {"페이지를 불러오는 중 오류가 발생했습니다.\n잠시 후 다시 시도해 주세요."}
      </Text>
      <View style={styles.buttons}>
        {onRetry && (
          <Pressable onPress={onRetry} style={[styles.btn, styles.btnPrimary]}>
            <Text style={styles.btnText}>다시 시도</Text>
          </Pressable>
        )}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: "#fdf9ee",
    paddingHorizontal: 16,
  },
  icon: {
    fontSize: 60,
    fontWeight: "700",
    color: "#43b9d6",
  },
  title: {
    marginTop: 16,
    fontSize: 24,
    fontWeight: "600",
    color: "#231815",
    textAlign: "center",
  },
  subtitle: {
    marginTop: 12,
    fontSize: 16,
    color: "rgba(35,24,21,0.6)",
    textAlign: "center",
    lineHeight: 24,
  },
  buttons: {
    marginTop: 32,
    flexDirection: "row",
    gap: 12,
  },
  btn: {
    paddingHorizontal: 32,
    paddingVertical: 12,
    borderRadius: 25,
    borderWidth: 2,
    borderColor: "#231815",
    justifyContent: "center",
    alignItems: "center",
  },
  btnPrimary: {
    backgroundColor: "#43b9d6",
  },
  btnText: {
    fontSize: 16,
    fontWeight: "600",
    color: "#231815",
  },
});
