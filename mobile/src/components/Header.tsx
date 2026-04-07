// Extracted from: web/src/components/header.tsx
// Differences: React Navigation instead of react-router, no SubscriptionNudgeBanner (separate component)

import { useState } from "react";
import {
  View,
  Text,
  Pressable,
  Modal,
  StyleSheet,
  Image,
} from "react-native";
import { useNavigation } from "@react-navigation/native";
import type { NativeStackNavigationProp } from "@react-navigation/native-stack";
import { useAuth } from "../contexts/auth-context";

type RootStackParamList = {
  Landing: undefined;
  Latest: undefined;
  AllNews: undefined;
  MyPage: undefined;
  Admin: undefined;
  Login: undefined;
};

type NavProp = NativeStackNavigationProp<RootStackParamList>;

interface HeaderProps {
  onLoginPress?: () => void;
}

export function Header({ onLoginPress }: HeaderProps) {
  const { user, logout } = useAuth();
  const navigation = useNavigation<NavProp>();
  const [menuOpen, setMenuOpen] = useState(false);

  const handleLogout = async () => {
    setMenuOpen(false);
    await logout();
    navigation.reset({ index: 0, routes: [{ name: "Landing" }] });
  };

  return (
    <View style={styles.wrapper}>
      <View style={styles.container}>
        {/* Logo */}
        {/* TODO: replace icon.png with wl-logo.png once brand assets are added to mobile/assets/ */}
        <Pressable onPress={() => navigation.navigate("Landing")}>
          <Image
            source={require("../../assets/icon.png")}
            style={styles.logo}
            resizeMode="contain"
          />
        </Pressable>

        {/* Right side */}
        <View style={styles.rightActions}>
          {user ? (
            <>
              <Pressable
                onPress={() => { setMenuOpen(false); navigation.navigate("MyPage"); }}
                style={styles.navBtn}
              >
                <Text style={styles.navBtnText}>마이페이지</Text>
              </Pressable>
            </>
          ) : (
            <Pressable
              onPress={onLoginPress}
              style={[styles.navBtn, styles.loginBtn]}
            >
              <Text style={styles.navBtnText}>로그인</Text>
            </Pressable>
          )}

          {/* Hamburger */}
          <Pressable
            onPress={() => setMenuOpen((v) => !v)}
            style={styles.hamburger}
            accessibilityLabel="메뉴"
          >
            <View style={styles.hamburgerLine} />
            <View style={styles.hamburgerLine} />
            <View style={styles.hamburgerLine} />
          </Pressable>
        </View>
      </View>

      {/* Mobile menu modal */}
      <Modal
        visible={menuOpen}
        transparent
        animationType="fade"
        onRequestClose={() => setMenuOpen(false)}
      >
        <Pressable style={styles.menuOverlay} onPress={() => setMenuOpen(false)}>
          <View style={styles.menuPanel}>
            <Pressable
              onPress={() => { setMenuOpen(false); navigation.navigate("Latest"); }}
              style={styles.menuItem}
            >
              <Text style={styles.menuItemText}>최신 소식 보기</Text>
            </Pressable>
            <Pressable
              onPress={() => { setMenuOpen(false); navigation.navigate("AllNews"); }}
              style={styles.menuItem}
            >
              <Text style={styles.menuItemText}>모든 소식 보기</Text>
            </Pressable>
            {user ? (
              <>
                {user.role === "admin" && (
                  <Pressable
                    onPress={() => { setMenuOpen(false); navigation.navigate("Admin"); }}
                    style={styles.menuItem}
                  >
                    <Text style={styles.menuItemText}>관리자</Text>
                  </Pressable>
                )}
                <Pressable
                  onPress={() => { setMenuOpen(false); navigation.navigate("MyPage"); }}
                  style={styles.menuItem}
                >
                  <Text style={styles.menuItemText}>마이페이지</Text>
                </Pressable>
                <Pressable onPress={handleLogout} style={styles.menuItem}>
                  <Text style={styles.menuItemText}>로그아웃</Text>
                </Pressable>
              </>
            ) : (
              <Pressable
                onPress={() => { setMenuOpen(false); onLoginPress?.(); }}
                style={styles.menuItem}
              >
                <Text style={styles.menuItemText}>로그인</Text>
              </Pressable>
            )}
          </View>
        </Pressable>
      </Modal>
    </View>
  );
}

const styles = StyleSheet.create({
  wrapper: {
    backgroundColor: "#fdf9ee",
    borderBottomWidth: 3,
    borderBottomColor: "#231815",
    zIndex: 40,
  },
  container: {
    maxWidth: 1200,
    alignSelf: "stretch",
    paddingHorizontal: 16,
    height: 65,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
  },
  logo: {
    width: 140,
    height: 40,
  },
  rightActions: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
  },
  navBtn: {
    paddingHorizontal: 16,
    height: 36,
    borderRadius: 18,
    backgroundColor: "#43b9d6",
    borderWidth: 2,
    borderColor: "#231815",
    justifyContent: "center",
    alignItems: "center",
  },
  loginBtn: {},
  navBtnText: {
    fontSize: 14,
    fontWeight: "600",
    color: "#231815",
  },
  hamburger: {
    width: 36,
    height: 36,
    justifyContent: "center",
    alignItems: "center",
    gap: 5,
  },
  hamburgerLine: {
    width: 20,
    height: 2,
    backgroundColor: "#231815",
  },
  menuOverlay: {
    flex: 1,
    backgroundColor: "rgba(0,0,0,0.3)",
    justifyContent: "flex-start",
    alignItems: "flex-end",
  },
  menuPanel: {
    backgroundColor: "#fdf9ee",
    borderWidth: 2,
    borderColor: "#231815",
    borderTopWidth: 0,
    minWidth: 180,
    paddingVertical: 8,
  },
  menuItem: {
    paddingHorizontal: 20,
    paddingVertical: 12,
  },
  menuItemText: {
    fontSize: 16,
    fontWeight: "500",
    color: "#231815",
  },
});
