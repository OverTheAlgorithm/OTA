// Extracted from: web/src/components/footer.tsx
// Differences: no Link (uses Linking.openURL), no router-based cookie-policy link

import { useEffect, useState } from "react";
import { View, Text, Pressable, StyleSheet, Linking, Image } from "react-native";
import { api } from "../lib/api";
import type { Term } from "../lib/api";

interface FooterProps {
  compact?: boolean;
}

export function Footer({ compact = false }: FooterProps) {
  const [terms, setTerms] = useState<Term[]>([]);

  useEffect(() => {
    api.getActiveTerms().then(setTerms).catch(() => {});
  }, []);

  const privacyAndTerms = terms.filter(
    (t) =>
      t.title === "개인정보 처리방침 동의" ||
      t.title === "서비스 이용약관 동의",
  );

  const openUrl = (url: string) => {
    const fullUrl = url.match(/^https?:\/\//) ? url : `https://${url}`;
    Linking.openURL(fullUrl).catch(() => {});
  };

  if (compact) {
    return (
      <View style={[styles.footer, styles.footerCompact]}>
        {/* TODO: replace icon.png with wl-logo.png once brand assets are added to mobile/assets/ */}
        <Image
          source={require("../../assets/icon.png")}
          style={[styles.logo, styles.logoCompact]}
          resizeMode="contain"
        />
        <View style={styles.links}>
          {privacyAndTerms.map((t) => (
            <Pressable key={t.id} onPress={() => openUrl(t.url)}>
              <Text style={[styles.linkText, styles.linkTextCompact]}>
                {t.title.replace(/ 동의$/, "")}
              </Text>
            </Pressable>
          ))}
        </View>
        <Text style={[styles.contactText, styles.contactTextCompact]}>
          문의: contact@wizletter.kr
        </Text>
        <Text style={[styles.copyright, styles.copyrightCompact]}>
          © 2026 WizLetter. All rights reserved.
        </Text>
      </View>
    );
  }

  return (
    <View style={styles.footer}>
      {/* TODO: replace icon.png with wl-logo.png once brand assets are added to mobile/assets/ */}
      <Image
        source={require("../../assets/icon.png")}
        style={styles.logo}
        resizeMode="contain"
      />
      <View style={styles.businessInfo}>
        <Text style={styles.infoText}>
          사업자 등록번호: 798-08-03338
        </Text>
        <Text style={styles.infoText}>
          주소: 서울특별시 영등포구 여의대방로43다길 19, 1층 101호(신길동)
        </Text>
        <Text style={styles.infoText}>문의: contact@wizletter.kr</Text>
      </View>
      <View style={styles.links}>
        {privacyAndTerms.map((t) => (
          <Pressable key={t.id} onPress={() => openUrl(t.url)}>
            <Text style={styles.linkText}>
              {t.title.replace(/ 동의$/, "")}
            </Text>
          </Pressable>
        ))}
      </View>
      <Text style={styles.copyright}>
        © 2026 WizLetter. All rights reserved.
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  footer: {
    borderTopWidth: 3,
    borderTopColor: "#231815",
    paddingVertical: 40,
    paddingHorizontal: 24,
    backgroundColor: "#fdf9ee",
    alignItems: "center",
    gap: 16,
  },
  footerCompact: {
    paddingVertical: 24,
    gap: 12,
  },
  logo: {
    width: 220,
    height: 48,
    opacity: 0.6,
  },
  logoCompact: {
    width: 160,
    height: 36,
  },
  businessInfo: {
    alignItems: "center",
    gap: 2,
  },
  infoText: {
    fontSize: 11,
    color: "rgba(35,24,21,0.6)",
    textAlign: "center",
  },
  links: {
    flexDirection: "row",
    flexWrap: "wrap",
    justifyContent: "center",
    gap: 16,
  },
  linkText: {
    fontSize: 14,
    color: "rgba(35,24,21,0.6)",
  },
  linkTextCompact: {
    fontSize: 12,
  },
  contactText: {
    fontSize: 12,
    color: "rgba(35,24,21,0.6)",
  },
  contactTextCompact: {},
  copyright: {
    fontSize: 14,
    color: "rgba(35,24,21,0.5)",
  },
  copyrightCompact: {
    fontSize: 12,
  },
});
